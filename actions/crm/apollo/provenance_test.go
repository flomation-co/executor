package apollo_common

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func person(fields map[string]interface{}, org map[string]interface{}) map[string]interface{} {
	m := map[string]interface{}{}
	for k, v := range fields {
		m[k] = v
	}
	if org != nil {
		m["organization"] = org
	}
	return m
}

// The person's own city and their employer's city are separate claims. The raw
// record nests the employer's under "organization", which is easy to read as
// confirming the individual — so the summary must state them apart.
func TestPeopleProvenance_SeparatesPersonFromCompanyLocation(t *testing.T) {
	RegisterTestingT(t)

	out := PeopleProvenance([]map[string]interface{}{
		person(map[string]interface{}{
			"first_name": "Ceri", "last_name": "Henfrey",
			"title": "Chief Operations Officer",
			"city":  "Wrexham", "state": "Wales", "country": "United Kingdom",
			"email": "ceri@example.com", "email_status": "verified",
		}, map[string]interface{}{
			"name": "Moneypenny", "city": "Wrexham", "country": "United Kingdom",
		}),
	})

	Expect(out).To(ContainSubstring("Ceri Henfrey"))
	Expect(out).To(ContainSubstring("person location: Wrexham, Wales, United Kingdom"))
	Expect(out).To(ContainSubstring("company location: Wrexham, United Kingdom"))
	Expect(out).To(ContainSubstring("status: verified"))
}

// A person with no city of their own must read as UNKNOWN, not inherit their
// employer's. This is the exact conflation that turns "company HQ is in
// Wrexham" into a false claim that the individual is local.
func TestPeopleProvenance_MissingPersonCityIsNotInherited(t *testing.T) {
	RegisterTestingT(t)

	out := PeopleProvenance([]map[string]interface{}{
		person(map[string]interface{}{
			"first_name": "Alyce", "last_name": "Kelso",
			"title": "Head of Operations & Strategy",
		}, map[string]interface{}{
			"name": "Moneypenny", "city": "Wrexham", "country": "United Kingdom",
		}),
	})

	Expect(out).To(ContainSubstring("person location: not provided"))
	Expect(out).To(ContainSubstring("company location: Wrexham, United Kingdom"))
	// Email absent must be stated, never blank-and-ambiguous.
	Expect(out).To(ContainSubstring("email: not provided"))
}

// A plan-withheld surname must be visible as withheld in the line itself, not
// silently rendered as a first name only.
func TestPeopleProvenance_FlagsObfuscatedSurname(t *testing.T) {
	RegisterTestingT(t)

	out := PeopleProvenance([]map[string]interface{}{
		person(map[string]interface{}{
			"first_name": "Becky", "last_name_obfuscated": "W***n",
			"title": "Head of Revenue & Sales Operations",
		}, map[string]interface{}{"name": "Moneypenny"}),
	})

	Expect(out).To(ContainSubstring("SURNAME WITHHELD BY PLAN"))
	Expect(out).To(ContainSubstring("W***n"))
}

func TestPeopleProvenance_EmptyAndMalformed(t *testing.T) {
	RegisterTestingT(t)

	Expect(PeopleProvenance(nil)).To(Equal(""))
	Expect(PeopleProvenance([]map[string]interface{}{})).To(Equal(""))

	// A record with nothing usable must still produce a line rather than panic,
	// so a malformed result is visible rather than silently dropped.
	out := PeopleProvenance([]map[string]interface{}{{}})
	Expect(out).To(ContainSubstring("(name not provided)"))
	Expect(strings.Count(out, "\n")).To(BeNumerically(">=", 2))
}

// Non-string values in the JSON (a number where a string was expected) must not
// panic the summary.
func TestPeopleProvenance_ToleratesNonStringFields(t *testing.T) {
	RegisterTestingT(t)

	out := PeopleProvenance([]map[string]interface{}{
		person(map[string]interface{}{
			"first_name": "Sam", "last_name": "Jones",
			"city": float64(42), "email": nil,
		}, nil),
	})
	Expect(out).To(ContainSubstring("Sam Jones"))
	Expect(out).To(ContainSubstring("person location: not provided"))
}
