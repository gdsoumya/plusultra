package model

import "fmt"

// Keys encode as quoted strings and numbers so they can key JSON maps.

// MarshalText implements encoding.TextMarshaler.
func (k ProgKey) MarshalText() ([]byte, error) {
	return fmt.Appendf(nil, "%q %q", k.Exercise, k.Label), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (k *ProgKey) UnmarshalText(b []byte) error {
	return scanKey(b, "%q %q", &k.Exercise, &k.Label)
}

// MarshalText implements encoding.TextMarshaler.
func (a SetAddr) MarshalText() ([]byte, error) {
	return fmt.Appendf(nil, "%q %q %d %d %d %d", a.Key.Exercise, a.Key.Label, a.Week, a.Day, a.Variation, a.Set), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (a *SetAddr) UnmarshalText(b []byte) error {
	return scanKey(b, "%q %q %d %d %d %d", &a.Key.Exercise, &a.Key.Label, &a.Week, &a.Day, &a.Variation, &a.Set)
}

// MarshalText implements encoding.TextMarshaler.
func (a DayAddr) MarshalText() ([]byte, error) {
	return fmt.Appendf(nil, "%q %q %d %d %d", a.Key.Exercise, a.Key.Label, a.Week, a.Day, a.Variation), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (a *DayAddr) UnmarshalText(b []byte) error {
	return scanKey(b, "%q %q %d %d %d", &a.Key.Exercise, &a.Key.Label, &a.Week, &a.Day, &a.Variation)
}

func scanKey(b []byte, format string, args ...any) error {
	if _, err := fmt.Sscanf(string(b), format, args...); err != nil {
		return fmt.Errorf("bad key %q: %w", b, err)
	}
	return nil
}
