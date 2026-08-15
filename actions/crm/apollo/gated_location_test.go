package apollo_common

import (
	"testing"

	. "github.com/onsi/gomega"
)

// The regression this whole file exists for.
//
// A Chester people search returned Practice Plan, Oxbury Bank, i6 Group and
// Exolum — genuinely Chester-area employers, i.e. the filter WORKED. But the
// key's plan masks personal data, so every person's own city came back null.
// Counting a withheld location as a non-match made every record "fail", the
// ignored-filter warning fired, and correctly-scoped results were reported as
// "location filter still ignored" and thrown away.
//
// Absence of evidence is not evidence of absence: a record with no location is
// unknown, not a miss.
func TestLocationIgnoredWarning_DoesNotFireWhenLocationsAreWithheld(t *testing.T) {
	RegisterTestingT(t)

	withheld := []string{"", "", "", "", "", "", "", "", "", ""}
	Expect(LocationIgnoredWarning([]string{"Chester, United Kingdom"}, withheld)).To(Equal(""))

	// Even a couple of stated locations is too thin to judge on.
	mostlyWithheld := []string{"", "", "", "London, England, United Kingdom", "", ""}
	Expect(LocationIgnoredWarning([]string{"Chester, United Kingdom"}, mostlyWithheld)).To(Equal(""))
}

// With enough STATED locations and none matching, it still fires — the Cheshire
// county case must not be silenced by this fix.
func TestLocationIgnoredWarning_StillFiresOnStatedNonMatches(t *testing.T) {
	RegisterTestingT(t)

	warn := LocationIgnoredWarning([]string{"Cheshire, United Kingdom"}, []string{
		"London, England, United Kingdom",
		"London, England, United Kingdom",
		"Slough, England, United Kingdom",
		"", // a withheld one must not inflate the denominator
	})

	Expect(warn).To(ContainSubstring("LOCATION FILTER MAY HAVE BEEN IGNORED"))
	// Counts only the records that stated a location: 3, not 4.
	Expect(warn).To(ContainSubstring("3 result(s)"))
}

// Person locations masked, employers in the requested area → the filter did
// take effect, but residency is unconfirmed. Both halves must be said: claiming
// the filter failed throws away good results, and saying nothing lets a company
// HQ pass as confirmation of where a person lives.
func TestPersonLocationNote_FallsBackToEmployerLocation(t *testing.T) {
	RegisterTestingT(t)

	note := PersonLocationNote(
		[]string{"Chester, United Kingdom"},
		[]string{"", "", "", "", ""},
		[]string{
			"Chester, England, United Kingdom", // Practice Plan
			"Chester, England, United Kingdom", // Oxbury Bank
			"Chester, England, United Kingdom", // i6 Group
			"London, England, United Kingdom",
			"",
		},
	)

	Expect(note).To(ContainSubstring("PERSON LOCATION WITHHELD, EMPLOYER LOCATION MATCHES"))
	Expect(note).To(ContainSubstring("3 of the 4"))
	Expect(note).To(ContainSubstring("UNCONFIRMED"))
	// Must not let a local employer stand in for a local person.
	Expect(note).To(ContainSubstring("does not make a given employee local"))
}

// Person locations masked AND no employer in the area → this really does look
// like national title-matching, and saying so is the useful answer.
func TestPersonLocationNote_WarnsWhenEmployersAreNotLocalEither(t *testing.T) {
	RegisterTestingT(t)

	note := PersonLocationNote(
		[]string{"Chester, United Kingdom"},
		[]string{"", "", "", ""},
		[]string{
			"London, England, United Kingdom",
			"Dublin, Ireland",
			"San Francisco, California, United States",
		},
	)

	Expect(note).To(ContainSubstring("LIKELY NOT LOCAL"))
	Expect(note).To(ContainSubstring("Treat these as unverified"))
}

// Nothing stated anywhere → say plainly that it cannot be determined, rather
// than guessing in either direction.
func TestPersonLocationNote_UnverifiableWhenEverythingIsWithheld(t *testing.T) {
	RegisterTestingT(t)

	note := PersonLocationNote(
		[]string{"Chester, United Kingdom"},
		[]string{"", "", "", ""},
		[]string{"", "", "", ""},
	)

	Expect(note).To(ContainSubstring("GEOGRAPHY UNVERIFIABLE"))
	Expect(note).To(ContainSubstring("cannot be determined"))
}

// When person locations ARE available, this must stay out of the way —
// LocationIgnoredWarning owns that case and two notices would just be noise.
func TestPersonLocationNote_SilentWhenPersonLocationsAreKnown(t *testing.T) {
	RegisterTestingT(t)

	Expect(PersonLocationNote(
		[]string{"Chester, United Kingdom"},
		[]string{"Chester, England, United Kingdom", "Ellesmere Port, England, United Kingdom", "Chester, England, United Kingdom"},
		[]string{"Chester, England, United Kingdom", "", ""},
	)).To(Equal(""))
}

func TestPersonLocationNote_SilentWithoutAFilterOrResults(t *testing.T) {
	RegisterTestingT(t)

	Expect(PersonLocationNote(nil, []string{"", ""}, []string{"", ""})).To(Equal(""))
	Expect(PersonLocationNote([]string{"Chester"}, nil, nil)).To(Equal(""))
}

// Search and enrichment withhold data for different reasons, so the advice must
// differ. Telling a search caller to set a reveal flag is unactionable — search
// has no such parameter — and telling them a masked surname is "usually not a
// plan problem" is simply wrong.
func TestGatePrefixSearch_GivesSearchSpecificAdvice(t *testing.T) {
	RegisterTestingT(t)

	recs := []map[string]interface{}{{"first_name": "Becky", "last_name_obfuscated": "W***n"}}

	search := GatePrefixSearch("Found 1 people", recs)
	Expect(search).To(ContainSubstring("Search NEVER returns a personal email"))
	Expect(search).To(ContainSubstring("IS a plan limitation"))
	Expect(search).To(ContainSubstring("free and Basic tiers mask personal data"))
	Expect(search).ToNot(ContainSubstring("Reveal Personal Emails ON by default"))

	enrich := GatePrefix("Enriched Becky", recs)
	Expect(enrich).To(ContainSubstring("USUALLY the reveal flag"))
	Expect(enrich).To(ContainSubstring("Reveal Personal Emails ON by default"))

	// Both stay silent on fully-revealed records.
	clean := []map[string]interface{}{{"first_name": "Ada", "last_name": "Lovelace", "email": "ada@x.com"}}
	Expect(GatePrefixSearch("Found 1 people", clean)).To(Equal("Found 1 people"))
	Expect(GatePrefix("Enriched Ada", clean)).To(Equal("Enriched Ada"))
}

func TestPeopleOrgLocations_ReadsTheNestedOrganisation(t *testing.T) {
	RegisterTestingT(t)

	got := PeopleOrgLocations([]map[string]interface{}{
		{"city": "Wrexham", "organization": map[string]interface{}{"city": "Chester", "country": "United Kingdom"}},
		{"city": "Leeds"},
		{},
	})
	// The PERSON's city must never leak into the employer column.
	Expect(got).To(Equal([]string{"Chester, United Kingdom", "", ""}))
}
