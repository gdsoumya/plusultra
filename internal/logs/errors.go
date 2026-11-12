// Package logs records what the user ate and their body check-ins.
package logs

import (
	"errors"
	"fmt"
)

// Sentinel errors for the service methods; the API layer maps them to statuses.
var (
	ErrNotFound  = errors.New("logs: not found")
	ErrInvalid   = errors.New("logs: invalid")
	ErrConflict  = errors.New("logs: already exists")
	ErrFinalized = errors.New("logs: workout record is finalized")
	// ErrUnknownProgram is the ErrInvalid for a program id that is not one of the user's.
	ErrUnknownProgram = fmt.Errorf("%w: unknown program", ErrInvalid)
)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}
