package pgvector

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	. "github.com/onsi/gomega"
)

// ---------------------------------------------------------------------------
// VectorLiteral
// ---------------------------------------------------------------------------

// The whole reason VectorLiteral exists: Go's default rendering of a float
// slice is space-separated ("[0.1 0.2]"), and pgvector rejects that outright.
// If anyone ever "simplifies" this to fmt.Sprintf("%v", vec), every write to
// the database starts failing — so pin the exact text form.
func TestVectorLiteral_CommaSeparatedNoSpaces(t *testing.T) {
	RegisterTestingT(t)

	got := VectorLiteral([]float32{0.1, 0.2})
	Expect(got).To(Equal("[0.1,0.2]"))
	Expect(got).ToNot(ContainSubstring(" "), "pgvector rejects a space-separated vector literal")

	// The thing we must never become.
	Expect(got).ToNot(Equal(fmt.Sprintf("%v", []float32{0.1, 0.2})))
}

func TestVectorLiteral_Shapes(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name string
		in   []float32
		want string
	}{
		{"empty slice", []float32{}, "[]"},
		{"nil slice", nil, "[]"},
		{"single element", []float32{1}, "[1]"},
		{"negative", []float32{-1.5, 2}, "[-1.5,2]"},
		{"three", []float32{0.1, 0.2, 0.3}, "[0.1,0.2,0.3]"},
		{"zero", []float32{0, 0}, "[0,0]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)
			Expect(VectorLiteral(tt.in)).To(Equal(tt.want))
		})
	}
}

// FormatFloat('g', -1, 32) emits the shortest decimal that round-trips a
// float32 exactly. That property is the reason for the -1 precision, so assert
// it bit-for-bit (not just ==, which would let -0 pass as +0) across the values
// most likely to break a naive formatter.
func TestVectorLiteral_RoundTripsExactly(t *testing.T) {
	RegisterTestingT(t)

	negZero := float32(math.Copysign(0, -1))

	vec := []float32{
		0.1,     // not representable in binary
		1e-7,    // exponent form
		-0.0,    // (this is +0 as a Go constant; negZero below is the real thing)
		negZero, // sign bit set on a zero
		1.0 / 3.0,
		3.4028235e38,  // max float32
		1.1754944e-38, // smallest normal float32
		-2.5,
		123456.78,
		0,
	}

	lit := VectorLiteral(vec)
	back, err := ParseVector(lit)
	Expect(err).ToNot(HaveOccurred())
	Expect(back).To(HaveLen(len(vec)))

	for i := range vec {
		Expect(math.Float32bits(back[i])).To(Equal(math.Float32bits(vec[i])),
			"element %d (%v) did not round-trip bit-for-bit through %q", i, vec[i], lit)
	}
}

// ---------------------------------------------------------------------------
// ParseVector
// ---------------------------------------------------------------------------

func TestParseVector(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name    string
		in      any
		want    []float32
		wantErr string
	}{
		// []byte is what lib/pq actually hands back for a `vector` column: the
		// extension's OID is assigned at CREATE EXTENSION time, so it can never
		// be in the driver's static decode switch.
		{"bytes from lib/pq", []byte("[0.1,0.2,0.3]"), []float32{0.1, 0.2, 0.3}, ""},
		{"string", "[0.1,0.2]", []float32{0.1, 0.2}, ""},
		{"nil", nil, nil, ""},
		{"empty literal", "[]", []float32{}, ""},
		{"empty literal bytes", []byte("[]"), []float32{}, ""},
		{"whitespace inside", " [ 0.1 , 0.2 ] ", []float32{0.1, 0.2}, ""},
		{"no brackets", "0.1,0.2", []float32{0.1, 0.2}, ""},
		{"negatives and exponents", "[-1.5,1e-7]", []float32{-1.5, 1e-7}, ""},
		{"malformed element", "[0.1,banana]", nil, "malformed vector element"},
		{"malformed empty element", "[0.1,,0.2]", nil, "malformed vector element"},
		{"unexpected type", 42, nil, "unexpected vector value of type int"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)
			got, err := ParseVector(tt.in)
			if tt.wantErr != "" {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.wantErr))
				return
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(got).To(Equal(tt.want))
		})
	}
}

// nil in must be nil out (not an empty slice) — a NULL vector column is a
// different thing from a zero-length one.
func TestParseVector_NilIsNilNotEmpty(t *testing.T) {
	RegisterTestingT(t)

	got, err := ParseVector(nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(got).To(BeNil())

	empty, err := ParseVector("[]")
	Expect(err).ToNot(HaveOccurred())
	Expect(empty).ToNot(BeNil())
	Expect(empty).To(HaveLen(0))
}

// ---------------------------------------------------------------------------
// CoerceVector
// ---------------------------------------------------------------------------

func TestCoerceVector(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name    string
		in      any
		want    []float32
		wantErr string
	}{
		{
			name: "already float32 (straight from another action)",
			in:   []float32{0.1, 0.2},
			want: []float32{0.1, 0.2},
		},
		{
			name: "float64",
			in:   []float64{0.1, 0.2},
			want: []float32{0.1, 0.2},
		},
		{
			// The JSON round-trip case: anything that went through the flow
			// store comes back as []any of float64.
			name: "[]any of float64",
			in:   []any{float64(0.1), float64(0.2)},
			want: []float32{0.1, 0.2},
		},
		{
			name: "[]any of mixed numeric kinds",
			in:   []any{float64(0.5), float32(0.25), int(1), int64(2)},
			want: []float32{0.5, 0.25, 1, 2},
		},
		{
			name: "[]any of json.Number",
			in:   []any{json.Number("0.1"), json.Number("0.2")},
			want: []float32{0.1, 0.2},
		},
		{
			name:    "[]any containing a non-number",
			in:      []any{float64(0.1), "banana"},
			wantErr: "embedding element 1 is a string, not a number",
		},
		{
			// The ${...} substitution case: the pass rewrites every reference
			// into its resolved *text*, so a vector arrives as a JSON string.
			name: "JSON array string",
			in:   `[0.1,0.2]`,
			want: []float32{0.1, 0.2},
		},
		{
			name: "JSON array string with whitespace",
			in:   "  [0.1, 0.2]  ",
			want: []float32{0.1, 0.2},
		},
		{
			// A pgvector literal that is NOT valid JSON — a user pasting the
			// value straight out of psql without the brackets.
			name: "bare pgvector literal",
			in:   "0.1,0.2",
			want: []float32{0.1, 0.2},
		},
		{
			name:    "empty string",
			in:      "",
			wantErr: "no embedding vector was supplied",
		},
		{
			name:    "nil",
			in:      nil,
			wantErr: "no embedding vector was supplied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)
			got, err := CoerceVector(tt.in)
			if tt.wantErr != "" {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.wantErr))
				return
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(got).To(Equal(tt.want))
		})
	}
}

// A wrong type is a wiring mistake, and the message has to say how to fix the
// wiring rather than naming a Go type at a front-of-house operator.
func TestCoerceVector_WrongTypeNamesTheFix(t *testing.T) {
	RegisterTestingT(t)

	_, err := CoerceVector(map[string]any{"embedding": []float64{0.1}})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Embed Text"),
		"the error must tell the operator to wire up an Embed Text step")
	Expect(err.Error()).To(ContainSubstring("map["))
}
