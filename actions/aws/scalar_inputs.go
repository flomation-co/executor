package aws

import (
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
)

// InputBool reads a boolean input, accepting a real bool value or a
// "true"/"1"/"yes"/"on" string (a ${var} substituted into a field arrives as
// text). Absent/unset → false.
func InputBool(name string, inputs []*core.Connection) bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return false
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	switch strings.ToLower(strings.TrimSpace(InputString(name, inputs))) {
	case "true", "1", "yes", "on":
		return true
	}
	return false
}

// InputInt reads an integer input robustly, accepting a native number (int/
// int64/float64 from a wired output or JSON) or a numeric string (a ${var}
// substituted into a string/integer field). The bool return is false when the
// input is absent, blank, or unparseable, so callers can leave an AWS field
// unset (nil pointer) rather than sending a spurious 0.
func InputInt(name string, inputs []*core.Connection) (int64, bool) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.Value == nil {
		return 0, false
	}
	switch v := c.Value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, false
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

// InputFloat reads a floating-point input (e.g. an Aurora Serverless v2 ACU
// capacity like 0.5 or 16), accepting a native number or a numeric string. The
// bool return is false when absent/blank/unparseable so callers leave the AWS
// field unset rather than sending 0.
func InputFloat(name string, inputs []*core.Connection) (float64, bool) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.Value == nil {
		return 0, false
	}
	switch v := c.Value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, false
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
