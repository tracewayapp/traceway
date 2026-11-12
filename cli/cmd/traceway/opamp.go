package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func readOpampJSON(cmd *cobra.Command, target any) error {
	path, _ := cmd.Flags().GetString("file")
	r := cmd.InOrStdin()
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		r = f
	}
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON document")
	}
	return nil
}

func opampUsageError(cmd *cobra.Command, err error) error {
	return renderUsageError(cmd.ErrOrStderr(), output.ResolveMode(flagOutput, output.StdoutIsTerminal()), err.Error(), cmd.CommandPath()+" --help")
}

func addOpampFileFlag(cmd *cobra.Command) {
	cmd.Flags().String("file", "", "JSON request file, or - for stdin")
	_ = cmd.MarkFlagRequired("file")
}

func newOpampCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "opamp", Short: "Browser SDK remote collection configuration"}
	rules := &cobra.Command{Use: "rules", Short: "Ordered remote collection rules; later matching rules win"}
	rules.AddCommand(&cobra.Command{
		Use: "list", Short: "List rules with their settings, conditions and updatedAt", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRaw(cmd, func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
				return c.ListSDKConfigRules(ctx, projectID)
			}, nil)
		},
	})
	preview := &cobra.Command{
		Use: "preview", Short: "Preview saved rules for client attributes, including plan limits; does not write", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var in client.SDKConfigPreviewInput
			if err := readOpampJSON(cmd, &in); err != nil {
				return opampUsageError(cmd, err)
			}
			return runRaw(cmd, func(ctx context.Context, c *client.Client, projectID string) (json.RawMessage, error) {
				return c.PreviewSDKConfig(ctx, projectID, in)
			}, nil)
		},
	}
	addOpampFileFlag(preview)
	cmd.AddCommand(rules, preview)
	return cmd
}
