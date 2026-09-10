package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/outbox"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"slices"
	"time"
)

const openThreadAdapter = "agent-open-thread"

type threadOpening struct {
	SchemaVersion int       `json:"schemaVersion"`
	AttemptId     uuid.UUID `json:"attemptId"`
	IntegrationId int       `json:"integrationId"`
	Origin        *Link     `json:"origin,omitempty"`
}

func queueThreads(tx *sql.Tx, attempt *models.AgentAttempt, origin *Link) error {
	integrations, err := transactional.IntegrationRepository.FindByOrganization(tx, attempt.OrganizationId)
	if err != nil {
		return err
	}
	for _, in := range integrations {
		if !in.Enabled || !slices.Contains(in.Kinds, KindChat) {
			continue
		}
		channel, ok := ChannelFor(in.Provider)
		if !ok {
			continue
		}
		defaultChannel, ok := channel.(DefaultChannel)
		if !ok || !defaultChannel.CanOpenDefaultThread() {
			continue
		}
		cfg, err := json.Marshal(threadOpening{SchemaVersion: EventSchemaVersion, AttemptId: attempt.Id, IntegrationId: in.Id, Origin: origin})
		if err != nil {
			return err
		}
		if _, err := outbox.Enqueue(tx, outbox.Delivery{Kind: models.OutboxKindAgent, AdapterType: openThreadAdapter, AdapterConfig: cfg, ProjectId: &attempt.ProjectId, ChannelName: in.Name, Message: models.NotificationMessage{Subject: fmt.Sprintf("Attempt %d", attempt.Number), Body: "Open the attempt conversation"}}); err != nil {
			return err
		}
	}
	return nil
}

func deliverThreadOpening(ctx context.Context, cfg json.RawMessage) error {
	var delivery threadOpening
	if err := json.Unmarshal(cfg, &delivery); err != nil {
		return err
	}
	if delivery.SchemaVersion != EventSchemaVersion {
		return fmt.Errorf("unsupported thread opening schema %d", delivery.SchemaVersion)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		// Serialize synchronous starts and outbox retries for the same attempt.
		if _, err := tx.ExecContext(ctx, "UPDATE agent_attempts SET updated_at = updated_at WHERE id = $1", delivery.AttemptId); err != nil {
			return struct{}{}, err
		}
		a, err := transactional.AgentAttemptRepository.FindById(tx, delivery.AttemptId)
		if err != nil {
			return struct{}{}, err
		}
		in, err := transactional.IntegrationRepository.FindById(tx, delivery.IntegrationId)
		if err != nil {
			return struct{}{}, err
		}
		if a == nil || in == nil || !in.Enabled || a.OrganizationId != in.OrganizationId {
			return struct{}{}, fmt.Errorf("attempt chat integration is unavailable")
		}
		links, err := transactional.AgentLinkRepository.FindByAttempt(tx, a.Id)
		if err != nil {
			return struct{}{}, err
		}
		for _, link := range links {
			if link.Kind == models.LinkKindThread && link.IntegrationId != nil && *link.IntegrationId == in.Id {
				return struct{}{}, nil
			}
		}
		channel, ok := ChannelFor(in.Provider)
		if !ok {
			return struct{}{}, fmt.Errorf("chat provider is unavailable")
		}
		var thread Link
		if requester, ok := channel.(ApprovalRequester); ok && a.Status == models.AttemptPendingApproval {
			thread, err = requester.RequestApproval(ctx, in, a)
		} else {
			thread, err = channel.Open(ctx, in, a, delivery.Origin)
		}
		if err != nil {
			return struct{}{}, err
		}
		thread.IntegrationId = in.Id
		link, err := RecordLink(tx, a.Id, thread, time.Now().UTC())
		if err != nil {
			return struct{}{}, err
		}
		after := 0
		for {
			messages, err := transactional.AgentMessageRepository.ListAfter(tx, a.Id, after, "", 200)
			if err != nil {
				return struct{}{}, err
			}
			for _, row := range messages {
				after = row.Id
				if row.Provider == thread.Provider {
					continue
				}
				if err := enqueueMirror(tx, a, link, Message{AttemptId: row.AttemptId, Provider: row.Provider, Direction: row.Direction, Kind: row.Kind, Body: row.Body, CreatedAt: row.CreatedAt}); err != nil {
					return struct{}{}, err
				}
			}
			if len(messages) < 200 {
				break
			}
		}
		return struct{}{}, nil
	})
	return err
}
