package core

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestSubstitutionString_StringPassesThrough(t *testing.T) {
	RegisterTestingT(t)
	Expect(substitutionString("London")).To(Equal("London"))
	Expect(substitutionString("")).To(Equal(""))
	Expect(substitutionString("with \"quotes\"")).To(Equal("with \"quotes\""))
}

func TestSubstitutionString_NilEmpty(t *testing.T) {
	RegisterTestingT(t)
	Expect(substitutionString(nil)).To(Equal(""))
}

func TestSubstitutionString_NumbersAndBools(t *testing.T) {
	RegisterTestingT(t)
	Expect(substitutionString(42)).To(Equal("42"))
	Expect(substitutionString(3.14)).To(Equal("3.14"))
	Expect(substitutionString(true)).To(Equal("true"))
	Expect(substitutionString(false)).To(Equal("false"))
}

func TestSubstitutionString_SliceEncodesAsJSON(t *testing.T) {
	RegisterTestingT(t)

	steps := []map[string]interface{}{
		{"instruction": "Head north", "distance_metres": 500.0, "duration_seconds": 60.0},
		{"instruction": "Continue on M40", "distance_metres": 100000.0, "duration_seconds": 3600.0},
	}
	got := substitutionString(steps)

	// Must be valid JSON that downstream actions can re-parse.
	Expect(got).To(ContainSubstring("\"instruction\":\"Head north\""))
	Expect(got).To(ContainSubstring("\"distance_metres\":500"))
	Expect(got).To(HavePrefix("["))
	Expect(got).To(HaveSuffix("]"))
}

func TestSubstitutionString_MapEncodesAsJSON(t *testing.T) {
	RegisterTestingT(t)

	got := substitutionString(map[string]interface{}{
		"lat": 51.5,
		"lng": -0.12,
	})

	Expect(got).To(ContainSubstring("\"lat\":51.5"))
	Expect(got).To(ContainSubstring("\"lng\":-0.12"))
	Expect(got).To(HavePrefix("{"))
	Expect(got).To(HaveSuffix("}"))
}

func TestSubstitutionString_StructEncodesAsJSON(t *testing.T) {
	RegisterTestingT(t)

	type point struct {
		Lat float64 `json:"latitude"`
		Lng float64 `json:"longitude"`
	}
	got := substitutionString(point{Lat: 51.5, Lng: -0.12})
	Expect(got).To(Equal(`{"latitude":51.5,"longitude":-0.12}`))
}
