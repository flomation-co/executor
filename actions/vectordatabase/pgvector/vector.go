package pgvector

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// This file is the whole of the `vector` type's wire handling, in both
// directions.
//
// pgvector's `vector` OID is assigned at CREATE EXTENSION time, so it is
// different on every database and can never appear in lib/pq's static oid.T_*
// decode switch. Values therefore fall straight through the driver's type
// switch and arrive as a raw []byte holding pgvector's *text* form —
// "[0.1,0.2,0.3]" — and the same text form is what the server accepts on the
// way back in. So the driver gives us no codec and we write both halves here.
//
// The upside of the text form is that a vector binds as an ordinary string
// parameter: `$1::vector`. It never has to be interpolated into SQL, which is
// why there is no injection surface on the one input that carries thousands of
// attacker-influenced floats.

// VectorLiteral renders a vector in the text form pgvector accepts.
//
// Do not reach for fmt.Sprintf("%v", vec) here: Go renders a float slice as
// "[0.1 0.2]" — space-separated, which pgvector rejects. Format 'g' with
// precision -1 emits the shortest decimal that round-trips the float32
// exactly, so a vector survives a store/load cycle bit-for-bit.
func VectorLiteral(v []float32) string {
	var b strings.Builder
	b.Grow(len(v)*12 + 2)
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// ParseVector decodes what lib/pq hands back for a `vector` column.
func ParseVector(src any) ([]float32, error) {
	var s string
	switch v := src.(type) {
	case nil:
		return nil, nil
	case []byte:
		s = string(v)
	case string:
		s = v
	default:
		return nil, fmt.Errorf("unexpected vector value of type %T", src)
	}

	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if strings.TrimSpace(s) == "" {
		return []float32{}, nil
	}

	parts := strings.Split(s, ",")
	out := make([]float32, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("malformed vector element %q", p)
		}
		out[i] = float32(f)
	}
	return out, nil
}

// CoerceVector accepts every shape an embedding can arrive in.
//
// The Embedding input is a ConnectionTypeObject fed by a ${...} reference to an
// upstream Embed Text step, and what actually lands in Value depends on how the
// value travelled: a real []float64 when it came straight from another action in
// the same run, a []any of float64 after a JSON round-trip through the flow
// store, or a string when the reference passed through the substitution pass
// (which rewrites every ${...} into its resolved *text*). All three are the
// same vector and all three must work — an operator who wired the node up
// correctly should never see a type error.
func CoerceVector(val any) ([]float32, error) {
	switch v := val.(type) {
	case nil:
		return nil, errors.New("no embedding vector was supplied")

	case []float32:
		return v, nil

	case []float64:
		out := make([]float32, len(v))
		for i, f := range v {
			out[i] = float32(f)
		}
		return out, nil

	case []any:
		out := make([]float32, len(v))
		for i, e := range v {
			switch n := e.(type) {
			case float64:
				out[i] = float32(n)
			case float32:
				out[i] = n
			case int:
				out[i] = float32(n)
			case int64:
				out[i] = float32(n)
			case json.Number:
				f, err := n.Float64()
				if err != nil {
					return nil, fmt.Errorf("embedding element %d (%q) is not a number", i, n.String())
				}
				out[i] = float32(f)
			default:
				return nil, fmt.Errorf("embedding element %d is a %T, not a number", i, e)
			}
		}
		return out, nil

	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil, errors.New("no embedding vector was supplied")
		}
		var arr []float64
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			out := make([]float32, len(arr))
			for i, f := range arr {
				out[i] = float32(f)
			}
			return out, nil
		}
		// Not JSON — try pgvector's own literal form, which also covers a bare
		// "0.1,0.2" that a user pasted without brackets.
		return ParseVector(s)
	}

	return nil, fmt.Errorf(
		"the Embedding input is a %T — connect it to an Embed Text step and pick its Embedding output", val)
}
