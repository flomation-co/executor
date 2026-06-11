package arithmetic_common

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
)

// ParseNumber pulls a float64 out of any Connection shape. Accepts ints,
// floats, strings ("5", "3.14", "-2e3"), and pointers thereto. Returns a
// descriptive error naming the input so the node's failure message points
// at the right field.
//
// Why float64 rather than int64: when a user wires a route's
// distance_miles output (string "234.7") into a multiplication node,
// the previous Integer-only ParseInt path failed with nil and the
// dereference panicked the executor. Floats accept both integer and
// decimal inputs without surprise.
func ParseNumber(c *core.Connection, fieldName string) (float64, error) {
	if c == nil {
		return 0, fmt.Errorf("arithmetic: input %q is missing", fieldName)
	}
	if c.Value == nil {
		return 0, fmt.Errorf("arithmetic: input %q has no value", fieldName)
	}
	switch v := c.Value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, fmt.Errorf("arithmetic: input %q is empty", fieldName)
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("arithmetic: input %q is not a number: %q", fieldName, s)
		}
		return f, nil
	}
	// Last-ditch: %v then re-parse. Handles odd Connection.Value types
	// that aren't worth special-casing (e.g. json.Number).
	s := strings.TrimSpace(fmt.Sprintf("%v", c.Value))
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("arithmetic: input %q is not a number: %v", fieldName, c.Value)
	}
	return f, nil
}

// FormatNumber renders a float64 in the most natural form — whole
// numbers as "5" rather than "5.000000", decimals trimmed to remove
// trailing zeros ("3.14" not "3.140000"), and IEEE-754 round-off noise
// suppressed (200.08 × 0.4 = 80.032, not 80.03200000000001).
func FormatNumber(f float64) string {
	if math.Trunc(f) == f && !math.IsInf(f, 0) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	// Round to 10 decimal places then strip trailing zeros. 10 places
	// is enough for practical work but cleans up the typical 1-bit
	// IEEE-754 representation drift.
	s := strconv.FormatFloat(f, 'f', 10, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}
