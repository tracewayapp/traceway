package agentrunner

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	traceway "go.tracewayapp.com"
	"time"
)

type tokenRenewer interface {
	RenewRunToken(context.Context, uuid.UUID) (*agent.RunToken, error)
}

func (l Local) RenewRunToken(ctx context.Context, id uuid.UUID) (*agent.RunToken, error) {
	return db.ExecuteTransaction(func(tx *sql.Tx) (*agent.RunToken, error) {
		if _, err := l.claimed(tx, id); err != nil {
			return nil, err
		}
		return agent.MintRunToken(tx, id)
	})
}

func (r *Runner) renewRunToken(ctx context.Context, stopRun context.CancelFunc, attempt *models.AgentAttempt, prep *preparation) func() {
	renewer, ok := r.Claimer.(tokenRenewer)
	if !ok {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		expires := prep.inputs.RunTokenExpiresAt
		for {
			wait := time.Until(expires.Add(-2 * time.Minute))
			if wait < time.Second {
				wait = time.Second
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			token, err := renewer.RenewRunToken(ctx, attempt.Id)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				traceway.CaptureException(fmt.Errorf("renew attempt token: %w", err))
				stopRun()
				return
			}
			if err := writeCLIProfile(prep.homeDir, r.InstanceURL, token.Token, attempt.ProjectId.String()); err != nil {
				traceway.CaptureException(fmt.Errorf("write renewed attempt token: %w", err))
				stopRun()
				return
			}
			expires = token.ExpiresAt
		}
	}()
	return func() { cancel(); <-done }
}
