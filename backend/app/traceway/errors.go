package traceway

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// StackTraceError carries the stack of the place it was created. Its type
// name and frame format match the retired native SDK on purpose: Traceway
// hashes the formatted stack, so a change here would split every existing
// issue of the self-monitoring project in two.
type StackTraceError struct {
	Err    error           `json:"-"`
	Frames []runtime.Frame `json:"-"`
}

func (e *StackTraceError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return ""
}

func (e *StackTraceError) Unwrap() error {
	return e.Err
}

func NewStackTraceErrorf(format string, args ...any) *StackTraceError {
	err := fmt.Errorf(format, args...)

	var existing *StackTraceError
	if errors.As(err, &existing) {
		return &StackTraceError{Err: err, Frames: existing.Frames}
	}

	return &StackTraceError{Err: err, Frames: captureStack(1)}
}

// captureStack(1) called from F returns frames starting at F's caller.
func captureStack(skip int) []runtime.Frame {
	const maxDepth = 64
	pcs := make([]uintptr, maxDepth)

	n := runtime.Callers(skip+2, pcs)
	if n == 0 {
		return nil
	}

	frames := runtime.CallersFrames(pcs[:n])
	result := make([]runtime.Frame, 0, n)
	for {
		frame, more := frames.Next()
		result = append(result, frame)
		if !more {
			break
		}
	}
	return result
}

// The first frame is the code that panicked, whatever recovery helpers sit
// above runtime.gopanic.
func panicStack() []runtime.Frame {
	frames := captureStack(1)
	for i := len(frames) - 1; i >= 0; i-- {
		if frames[i].Function == "runtime.gopanic" {
			return frames[i+1:]
		}
	}
	return frames
}

func formatFrames(frames []runtime.Frame) string {
	var sb strings.Builder
	for _, frame := range frames {
		fn := frame.Function
		if idx := strings.LastIndex(fn, "/"); idx >= 0 {
			fn = fn[idx+1:]
		}
		if idx := strings.Index(fn, "."); idx >= 0 {
			fn = fn[idx+1:]
		}
		fmt.Fprintf(&sb, "%s()\n", fn)
		fmt.Fprintf(&sb, "    %s:%d\n", frame.File, frame.Line)
	}
	return sb.String()
}

func errorType(err error) string {
	return reflect.TypeOf(err).String()
}

func panicError(r any) error {
	if err, ok := r.(error); ok {
		return err
	}
	return fmt.Errorf("%v", r)
}

func FormatWithStack(err error, frames []runtime.Frame) string {
	return fmt.Sprintf("%s: %s\n%s", errorType(err), err.Error(), formatFrames(frames))
}
