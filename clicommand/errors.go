package clicommand

import (
	"errors"
	"fmt"
	"os"
)

// ExitError is used to signal to main.go that the command should exit with a
// the exit code in `code`.
type ExitError struct {
	code  int
	inner error
}

func NewExitError(code int, err error) *ExitError {
	return &ExitError{code: code, inner: err}
}

func (e *ExitError) Code() int {
	return e.code
}

func (e *ExitError) Error() string {
	return e.inner.Error()
}

func (e *ExitError) Unwrap() error {
	return e.inner
}

func (e *ExitError) Is(target error) bool {
	terr, ok := target.(*ExitError)
	return ok && e.code == terr.code
}

// SilentExitError instructs the main method to not log anything and just exit
// with status `code`
type SilentExitError struct {
	code int
}

func NewSilentExitError(code int) *SilentExitError {
	return &SilentExitError{code: code}
}

// Error prints the exit code, but should not be used in main.go
func (e *SilentExitError) Error() string {
	return fmt.Sprintf("silently exited status %d", e.code)
}

func (e *SilentExitError) Code() int {
	return e.code
}

func (e *SilentExitError) Is(target error) bool {
	terr, ok := target.(*SilentExitError)
	return ok && e.code == terr.code
}

func ErrToExitCode(err error) int {
	if serr := new(SilentExitError); errors.As(err, &serr) {
		return serr.Code()
	}

	fmt.Fprintf(os.Stderr, "fatal: %v\n", err)

	if eerr := new(ExitError); errors.As(err, &eerr) {
		return eerr.Code()
	}

	return 1
}
