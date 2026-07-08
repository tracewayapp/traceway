package main

import (
	"errors"
	"io"

	"github.com/tracewayapp/traceway/cli/internal/errclass"
	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/internal/output"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// cliError carries an exit code alongside an error message. Command runners
// return *cliError to communicate which exitcode the CLI should terminate
// with. main() type-asserts on the returned error to extract the code.
type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }

// newCLIError is a small constructor.
func newCLIError(code int, sentinel string) *cliError {
	return &cliError{code: code, err: errors.New(sentinel)}
}

// renderAPIError writes the appropriate envelope to errOut and returns a
// sentinel error so the cobra runner sees a non-nil result. The actual exit
// code is communicated via the envelope's ExitCode field; main() resolves it.
//
// loginContext = true means we're in the login command itself; an Unauthorized
// from there means "wrong username/password", not "session expired".
func renderAPIError(errOut io.Writer, mode output.Mode, err error, loginContext bool) error {
	env := classifyError(err, loginContext)
	_ = output.RenderError(errOut, mode, env)
	return newCLIError(env.ExitCode, env.Code)
}

// classifyError delegates to internal/errclass (shared with the MCP server)
// and layers on the CLI-only context: a login-command 401 means bad
// credentials, and an explicit --profile belongs in the re-login hint.
func classifyError(err error, loginContext bool) output.ErrorEnvelope {
	if loginContext && errors.Is(err, client.ErrUnauthorized) {
		return output.ErrorEnvelope{
			Code: "not_authenticated", Message: "invalid email or password",
			ExitCode: exitcode.Auth,
		}
	}
	c := errclass.Classify(err)
	if c.Code == "token_expired" && flagProfile != "" {
		c.Hint = "traceway login --profile " + flagProfile
	}
	return output.ErrorEnvelope{
		Code:     c.Code,
		Message:  c.Message,
		Hint:     c.Hint,
		ExitCode: c.ExitCode,
	}
}
