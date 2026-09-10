package access

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound is returned when a ref, id or hash does not exist at the
	// source. The Traceway client maps its 404 to this same value.
	ErrNotFound = errors.New("not found (404)")
	// ErrUnsupported is returned by a method the source's Capabilities say
	// it cannot answer.
	ErrUnsupported = errors.New("the source does not support this query")
	// ErrUnknownProvider is returned by Open for a provider nobody registered.
	ErrUnknownProvider = errors.New("unknown provider")
	// ErrUnknownSource is returned when a name matches no configured source.
	ErrUnknownSource = errors.New("unknown source")
	// ErrNoSource is returned when no configured source answers a domain.
	ErrNoSource = errors.New("no source answers this domain")
)

// SourceError is one source's failure inside a fan-out.
type SourceError struct {
	Source string
	Err    error
}

func (e *SourceError) Error() string { return fmt.Sprintf("source %s: %v", e.Source, e.Err) }
func (e *SourceError) Unwrap() error { return e.Err }
