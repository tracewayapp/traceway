// Package errclass maps pkg/client errors to a stable code/message/hint
// classification. The CLI wraps a Classified into its error envelope (adding
// context-specific hints and the exit code); the MCP server renders it as a
// tool error. Keeping the mapping here means both surfaces branch on the same
// snake_case codes.
package errclass

import (
	"errors"
	"net/url"

	"github.com/tracewayapp/traceway/cli/internal/exitcode"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// Classified is the surface-agnostic classification of a client error.
type Classified struct {
	Code     string
	Message  string
	Hint     string
	ExitCode int
}

// Classify maps a pkg/client error to its classification. The token_expired
// hint is the plain "traceway login"; callers with more context (an explicit
// --profile, a login command, an MCP session) override it.
func Classify(err error) Classified {
	switch {
	case errors.Is(err, client.ErrUnauthorized):
		return Classified{
			Code: "token_expired", Message: "session expired or invalid",
			Hint:     "traceway login",
			ExitCode: exitcode.Auth,
		}
	case errors.Is(err, client.ErrForbidden):
		return Classified{
			Code: "forbidden", Message: "permission denied",
			ExitCode: exitcode.Auth,
		}
	case errors.Is(err, client.ErrNotFound):
		return Classified{
			Code: "not_found", Message: "resource not found",
			ExitCode: exitcode.NotFound,
		}
	case errors.Is(err, client.ErrRateLimited):
		return Classified{
			Code: "rate_limited", Message: "rate limit exceeded — slow down or retry later",
			ExitCode: exitcode.RateLimited,
		}
	}
	if apiErr, ok := errors.AsType[*client.APIError](err); ok {
		if apiErr.StatusCode >= 500 {
			return Classified{
				Code: "server_error", Message: apiErr.Error(),
				ExitCode: exitcode.Server,
			}
		}
		return Classified{
			Code: "api_error", Message: apiErr.Error(),
			ExitCode: exitcode.Generic,
		}
	}
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		return Classified{
			Code: "connection_failed", Message: urlErr.Error(),
			Hint:     "check that the Traceway URL is reachable and the network is up",
			ExitCode: exitcode.Connection,
		}
	}
	return Classified{
		Code: "internal", Message: err.Error(),
		ExitCode: exitcode.Generic,
	}
}
