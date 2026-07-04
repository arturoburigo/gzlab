package config

import (
	"fmt"
	"time"
)

// Duration wraps time.Duration so it round-trips through YAML as a string
// like "45s" or "24h" instead of yaml.v3's default (a raw nanosecond count).
type Duration time.Duration

func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}
