package journey_common

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

func RequiredString(name string, inputs []*core.Connection) (string, error) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil || strings.TrimSpace(*c.String()) == "" {
		return "", fmt.Errorf("journey: input %q is required", name)
	}
	return strings.TrimSpace(*c.String()), nil
}

func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return strings.TrimSpace(*c.String())
}

// OptionalCSV reads an input as a comma- or pipe-separated list. Used for
// waypoints and avoid arrays. Empty entries are dropped.
func OptionalCSV(name string, inputs []*core.Connection) []string {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil
	}
	sep := ","
	if strings.Contains(raw, "|") && !strings.Contains(raw, ",") {
		sep = "|"
	}
	parts := strings.Split(raw, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// OptionalTime accepts ISO 8601 / RFC 3339, or unix seconds. Returns nil if
// the input is blank. Returning nil signals "now" to the provider.
func OptionalTime(name string, inputs []*core.Connection) (*time.Time, error) {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", raw); err == nil {
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04", raw); err == nil {
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", raw); err == nil {
		return &t, nil
	}
	var unix int64
	if _, err := fmt.Sscanf(raw, "%d", &unix); err == nil && unix > 0 {
		t := time.Unix(unix, 0)
		return &t, nil
	}
	return nil, fmt.Errorf("journey: input %q must be RFC3339 or unix seconds, got %q", name, raw)
}

func ProviderInput(inputs []*core.Connection) (Provider, error) {
	name := OptionalString("provider", inputs)
	if name == "" {
		name = ProviderGoogle
	}
	apiKey, err := RequiredString("api_key", inputs)
	if err != nil {
		return nil, err
	}
	return NewProvider(name, apiKey)
}

// Note: Options arrays for provider/mode/etc inputs are inlined in each
// action's Inputs literal because the manifest generator only resolves
// composite literals, not function call results. Don't extract them.
