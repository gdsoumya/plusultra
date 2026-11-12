package sync

import (
	"errors"
	"testing"

	"github.com/gdsoumya/plusultra/internal/logs"
)

func TestOutcomeReasons(t *testing.T) {
	none := func() (*int, error) { return nil, nil }
	for err, want := range map[error]string{
		logs.ErrNotFound:       reasonOutdated, // deleted by another writer between the read and the write
		logs.ErrFinalized:      reasonFinalized,
		logs.ErrUnknownProgram: reasonUnknownProgram,
		logs.ErrInvalid:        reasonInvalid,
		logs.ErrConflict:       reasonInvalid, // no row holds the date: the id is another user's
	} {
		if row, reason, gotErr := outcome(err, none); row != nil || reason != want || gotErr != nil {
			t.Errorf("outcome(%v) = %v, %q, %v; want reason %q", err, row, reason, gotErr, want)
		}
	}
	other := 1
	if row, reason, _ := outcome(logs.ErrConflict, func() (*int, error) { return &other, nil }); row != &other || reason != reasonDateTaken {
		t.Errorf("date taken = %v, %q", row, reason)
	}
	boom := errors.New("boom")
	if _, _, err := outcome(boom, none); !errors.Is(err, boom) {
		t.Errorf("other errors pass through: %v", err)
	}
}
