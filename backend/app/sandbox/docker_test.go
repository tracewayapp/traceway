package sandbox

import (
	"context"
	"slices"
	"testing"
)

func TestDockerOverridesImageEntrypoint(t *testing.T) {
	d := &Docker{Image: "traceway:agent"}
	cmd, cleanup, err := d.Build(context.Background(), Spec{Command: []string{"claude", "-p"}, Network: NetworkPolicy{Mode: NetworkOff}})
	if err != nil {
		t.Fatal(err)
	}
	// No process was started; cancel only the context, without invoking Docker cleanup.
	_ = cleanup
	i := slices.Index(cmd.Args, "--entrypoint")
	if i < 0 || i+3 >= len(cmd.Args) || cmd.Args[i+1] != "claude" || cmd.Args[i+2] != "traceway:agent" || cmd.Args[i+3] != "-p" {
		t.Fatalf("argv = %v", cmd.Args)
	}
}

func TestDockerUnsupportedAllowlistFailsClosed(t *testing.T) {
	d := &Docker{Image: "traceway:agent"}
	if _, _, err := d.Build(context.Background(), Spec{Command: []string{"claude"}, Network: NetworkPolicy{Mode: NetworkAllow}}); err == nil {
		t.Fatal("unsupported allowlist silently allowed unrestricted egress")
	}
}
