package controllers

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/agentrunner"
	"github.com/tracewayapp/traceway/backend/app/integrations/github"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func buildAgentPreflight(ctx context.Context, tx *sql.Tx, projectId uuid.UUID, hash string, profileId *int) (*preflightResponse, error) {
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		return nil, err
	}
	if project == nil || project.OrganizationId == nil {
		return nil, errors.New("the project belongs to no organization")
	}
	orgId := *project.OrganizationId
	repository, err := transactional.RepositoryRepository.FindByProject(tx, projectId)
	if err != nil {
		return nil, err
	}
	profiles, err := transactional.AgentProfileRepository.FindByOrganization(tx, orgId)
	if err != nil {
		return nil, err
	}
	integrations, err := transactional.IntegrationRepository.FindByOrganization(tx, orgId)
	if err != nil {
		return nil, err
	}
	active, err := transactional.AgentAttemptRepository.FindActiveBySubject(tx, projectId, models.SubjectKindTracewayException, hash)
	if err != nil {
		return nil, err
	}
	nextNumber, err := transactional.AgentAttemptRepository.NextNumber(tx, projectId, models.SubjectKindTracewayException, hash)
	if err != nil {
		return nil, err
	}
	response := &preflightResponse{NextNumber: nextNumber, ActiveAttempt: active, Profiles: profiles}
	repositoryCheck := preflightCheck{Key: "repository", Label: "GitHub repository", Hint: "Connect GitHub and select the repository for this project.", Href: "/agent?tab=repository"}
	if repository != nil && repository.IntegrationId != nil {
		for _, integration := range integrations {
			if integration.Id != *repository.IntegrationId || integration.Provider != github.Provider || !integration.Enabled {
				continue
			}
			repositoryCheck.Ok = github.New().Ready(integration)
			if repositoryCheck.Ok {
				repositoryCheck.Label = "GitHub " + repository.Owner + "/" + repository.Name
				repositoryCheck.Hint = "Pull requests target " + repository.DefaultBranch + "."
			} else {
				repositoryCheck.Hint = "Finish connecting GitHub: install the GitHub App or configure a personal access token."
				repositoryCheck.Href = "/settings"
			}
			break
		}
	}
	profileCheck := preflightCheck{Key: "profile", Label: "Agent profile", Hint: "Choose an agent profile with a model provider credential in Settings.", Href: "/settings"}
	for _, profile := range profiles {
		if profile.IsDefault {
			id := profile.Id
			response.DefaultProfile = &id
		}
		if (profileId != nil && profile.Id == *profileId) || (profileId == nil && profile.IsDefault) {
			profileCheck.Ok = profile.Credential != ""
			profileCheck.Label = "Agent " + profile.Name + " (" + profile.Agent + describeModel(profile.Model) + ")"
			if profileCheck.Ok {
				profileCheck.Hint = ""
			}
		}
	}
	executorCheck := preflightCheck{Key: "executor", Label: "Traceway AI Agent", Hint: "Traceway AI Agent is not enabled on this instance.", Href: "https://docs.tracewayapp.com/learn/agent/runner"}
	mode, err := agentrunner.Mode()
	if err != nil {
		return nil, err
	}
	switch mode {
	case agentrunner.ModeEmbedded:
		executorCheck.Ok = true
		executorCheck.Hint = "This instance is configured to run fixes with its embedded agent."
	case agentrunner.ModeRemote:
		executorCheck.Ok = true
		executorCheck.Hint = "Fixes are queued for this instance's agent runners."
	}
	channelCheck := preflightCheck{Key: "channel", Ok: true, Label: "Conversation in the browser", Href: "/settings"}
	for _, integration := range integrations {
		if !integration.Enabled || !hasKind(integration.Kinds, agent.KindChat) {
			continue
		}
		channel, ok := agent.ChannelFor(integration.Provider)
		if !ok {
			continue
		}
		defaultChannel, ok := channel.(agent.DefaultChannel)
		if ok && defaultChannel.CanOpenDefaultThread() {
			channelCheck.Label = "Thread mirrored to " + integration.Provider + " (" + integration.Name + ")"
			break
		}
	}
	response.Checks = []preflightCheck{repositoryCheck, profileCheck, executorCheck, channelCheck}
	response.CanStart = repositoryCheck.Ok && profileCheck.Ok && executorCheck.Ok && active == nil
	return response, nil
}
