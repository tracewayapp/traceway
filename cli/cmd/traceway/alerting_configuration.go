package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tracewayapp/traceway/cli/v2/internal/output"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func addAlertingConfigurationCommands(root *cobra.Command, write bool) {
	groups := map[string]*cobra.Command{}
	for _, cmd := range root.Commands() {
		groups[cmd.Name()] = cmd
	}
	for _, op := range client.AlertingOperations() {
		if !op.ReadOnly && !write {
			continue
		}
		group := groups[op.Resource]
		if group == nil {
			group = &cobra.Command{Use: op.Resource, Short: "Manage " + op.Resource + " configuration"}
			root.AddCommand(group)
			groups[op.Resource] = group
		}
		found := false
		for _, sub := range group.Commands() {
			if sub.Name() == op.Action {
				found = true
				break
			}
		}
		if !found {
			group.AddCommand(newAlertingConfigurationCommand(op))
		}
	}
	for resource, group := range groups {
		supported := false
		for _, op := range client.AlertingOperations() {
			if op.Resource == resource && op.Body {
				supported = true
				break
			}
		}
		if supported {
			group.AddCommand(newAlertingValidateCommand(resource))
		}
	}
}

func alertingRequestFromCommand(cmd *cobra.Command, op client.AlertingOperation, args []string) (client.AlertingRequest, error) {
	var req client.AlertingRequest
	var err error
	req.OrganizationID, _ = cmd.Flags().GetString("organization")
	req.From, _ = cmd.Flags().GetString("from")
	req.To, _ = cmd.Flags().GetString("to")
	if op.ID {
		if req.ID, err = intArg(cmd, args[0], "id"); err != nil {
			return req, err
		}
	}
	if op.ChildID {
		if req.ChildID, err = intArg(cmd, args[1], "override id"); err != nil {
			return req, err
		}
	}
	if op.Body {
		if err = readJSONFile(cmd, &req.Body); err != nil {
			return req, jsonFileUsageError(cmd, err)
		}
	}
	return req, nil
}

func addAlertingFlags(cmd *cobra.Command, body, org, timeline bool) {
	if body {
		addJSONFileFlag(cmd)
	}
	if org {
		cmd.Flags().String("organization", "", "Organization ID (must own the selected project)")
	}
	if timeline {
		cmd.Flags().String("from", "", "Start time (RFC3339)")
		cmd.Flags().String("to", "", "End time (RFC3339, at most 62 days later)")
	}
}

func newAlertingConfigurationCommand(op client.AlertingOperation) *cobra.Command {
	use := op.Action
	count := 0
	if op.ID {
		use += " <id>"
		count++
	}
	if op.ChildID {
		use += " <override-id>"
		count++
	}
	cmd := &cobra.Command{Use: use, Short: op.Action + " " + op.Resource, Args: cobra.ExactArgs(count), RunE: func(cmd *cobra.Command, args []string) error {
		req, err := alertingRequestFromCommand(cmd, op, args)
		if err != nil {
			return err
		}
		// Validate before authentication or confirmation without requiring an online session.
		req.ProjectID = "selected-project"
		if err = client.ValidateAlertingRequest(op.Resource, op.Action, req); err != nil {
			return jsonFileUsageError(cmd, err)
		}
		call := func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
			req.ProjectID = projectID
			return c.ConfigureAlerting(ctx, op.Resource, op.Action, req)
		}
		if op.ReadOnly {
			return runRawWithProject(cmd, op.Scope == "project", call, nil)
		}
		summary := fmt.Sprintf("About to %s %s", op.Action, op.Resource)
		if op.ID {
			summary += fmt.Sprintf(" %d", req.ID)
		}
		if op.Scope == "organization" {
			summary += " in organization " + req.OrganizationID
		}
		if op.Scope == "user" {
			summary += " for your own account"
		}
		if op.Action == "update" || op.Action == "replace" || op.Action == "set-members" || op.Action == "set-projects" {
			summary += "; this replaces the submitted configuration"
		}
		if op.Resource == "channels" && op.Action == "delete" {
			summary += "; ALL attached notification rules are permanently deleted too"
		}
		if op.Action == "test" || op.Action == "resend-code" || (op.Resource == "contact-methods" && op.Action == "create") {
			summary += "; this may send a real notification"
		}
		return runAlertingMutation(cmd, []string{summary + "."}, map[string]any{"action": op.Action, "message": "Completed " + op.Resource + " " + op.Action + "."}, call)
	}}
	addAlertingFlags(cmd, op.Body, op.Scope == "organization", op.Action == "timeline")
	return cmd
}

func newAlertingValidateCommand(resource string) *cobra.Command {
	cmd := &cobra.Command{Use: "validate <action> [id] [override-id]", Short: "Validate JSON structure offline; server checks referenced entities and nested configuration", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		var selected *client.AlertingOperation
		for _, op := range client.AlertingOperations() {
			if op.Resource == resource && op.Action == args[0] {
				selected = &op
				break
			}
		}
		if selected == nil {
			return jsonFileUsageError(cmd, fmt.Errorf("unsupported %s action %q", resource, args[0]))
		}
		count := 1
		if selected.ID {
			count++
		}
		if selected.ChildID {
			count++
		}
		if len(args) != count {
			return jsonFileUsageError(cmd, fmt.Errorf("expected %d positional arguments", count))
		}
		req, err := alertingRequestFromCommand(cmd, *selected, args[1:])
		if err != nil {
			return err
		}
		req.ProjectID = "offline"
		if err = client.ValidateAlertingRequest(resource, selected.Action, req); err != nil {
			return jsonFileUsageError(cmd, err)
		}
		return renderRaw(cmd, output.ResolveMode(flagOutput, output.StdoutIsTerminal()), json.RawMessage(`{"valid":true,"validation":"structure only; server validates references and nested configuration"}`), nil)
	}}
	addAlertingFlags(cmd, true, true, true)
	return cmd
}
