package apollo_common

import (
	"testing"

	. "github.com/onsi/gomega"
)

// The Cheshire case: Apollo does not recognise the county, drops the filter and
// answers an unfiltered relevance search. The result is a plausible page of
// large national employers that reads as a successful county search.
func TestLocationIgnoredWarning_FiresWhenNothingMatches(t *testing.T) {
	RegisterTestingT(t)

	warn := LocationIgnoredWarning(
		[]string{"Cheshire, United Kingdom"},
		[]string{
			"London, England, United Kingdom", // The Economist
			"London, England, United Kingdom", // Financial Times
			"London, England, United Kingdom", // Robert Walters
			"Slough, England, United Kingdom", // Reckitt
			"Manchester, England, United Kingdom",
		},
	)

	Expect(warn).To(ContainSubstring("LOCATION FILTER MAY HAVE BEEN IGNORED"))
	Expect(warn).To(ContainSubstring("Cheshire, United Kingdom"))
	Expect(warn).To(ContainSubstring("5 result(s)"))
	// It must steer to the thing that actually works.
	Expect(warn).To(ContainSubstring("city"))
}

// Critical: Apollo's city filters legitimately include surrounding towns, so a
// Chester search returning Ellesmere Port and Capenhurst is CORRECT. Warning
// here would train the reader to distrust good results, which is worse than not
// warning at all.
func TestLocationIgnoredWarning_QuietOnGenuineLocalResults(t *testing.T) {
	RegisterTestingT(t)

	warn := LocationIgnoredWarning(
		[]string{"Chester, United Kingdom"},
		[]string{
			"Chester, England, United Kingdom",        // Practice Plan
			"Capenhurst, England, United Kingdom",     // EA Technology
			"Chester, England, United Kingdom",        // Oxbury Bank
			"Ellesmere Port, England, United Kingdom", // D Morgan plc
			"London, England, United Kingdom",         // BT Group — the outlier
		},
	)

	Expect(warn).To(Equal(""))
}

// One match is enough to prove the filter had an effect.
func TestLocationIgnoredWarning_QuietOnSingleMatch(t *testing.T) {
	RegisterTestingT(t)

	Expect(LocationIgnoredWarning(
		[]string{"Chester, United Kingdom"},
		[]string{"London, England, United Kingdom", "Leeds, England, United Kingdom", "Chester, England, United Kingdom"},
	)).To(Equal(""))
}

// The leading segment must match, since a record reports city and country in
// separate fields while the filter is written "City, Country".
func TestLocationIgnoredWarning_MatchesOnCitySegment(t *testing.T) {
	RegisterTestingT(t)

	Expect(LocationIgnoredWarning(
		[]string{"Chester, United Kingdom"},
		[]string{"Chester, England, United Kingdom", "x", "y"},
	)).To(Equal(""))

	// A country-level filter is satisfied by the country field.
	Expect(LocationIgnoredWarning(
		[]string{"United Kingdom"},
		[]string{"London, England, United Kingdom", "Leeds, England, United Kingdom", "Hull, England, United Kingdom"},
	)).To(Equal(""))
}

// Below the threshold a zero-match run is as likely to be a sparse area as an
// ignored filter, so it stays quiet rather than crying wolf.
func TestLocationIgnoredWarning_QuietOnTinyResultSets(t *testing.T) {
	RegisterTestingT(t)

	Expect(LocationIgnoredWarning([]string{"Cheshire, United Kingdom"}, []string{"London, England, United Kingdom"})).To(Equal(""))
	Expect(LocationIgnoredWarning([]string{"Cheshire, United Kingdom"}, []string{"London", "Leeds"})).To(Equal(""))
}

// No filter requested means nothing to judge.
func TestLocationIgnoredWarning_QuietWithoutAFilter(t *testing.T) {
	RegisterTestingT(t)

	Expect(LocationIgnoredWarning(nil, []string{"London", "Leeds", "Hull"})).To(Equal(""))
	Expect(LocationIgnoredWarning([]string{"  "}, []string{"London", "Leeds", "Hull"})).To(Equal(""))
	Expect(LocationIgnoredWarning([]string{"Chester"}, nil)).To(Equal(""))
}

func TestLocationIgnoredWarning_IsCaseInsensitive(t *testing.T) {
	RegisterTestingT(t)

	Expect(LocationIgnoredWarning(
		[]string{"CHESTER, United Kingdom"},
		[]string{"chester, england, united kingdom", "x", "y"},
	)).To(Equal(""))
}

// Multiple requested locations: a match on ANY of them means the filter worked.
func TestLocationIgnoredWarning_AnyRequestedLocationCounts(t *testing.T) {
	RegisterTestingT(t)

	Expect(LocationIgnoredWarning(
		[]string{"Chester, United Kingdom", "Wrexham, United Kingdom"},
		[]string{"Wrexham, Wales, United Kingdom", "London, England, United Kingdom", "Leeds, England, United Kingdom"},
	)).To(Equal(""))
}

func TestOrgAndPersonLocations(t *testing.T) {
	RegisterTestingT(t)

	recs := []map[string]interface{}{
		{"city": "Chester", "state": "England", "country": "United Kingdom"},
		{"country": "United Kingdom"},
		{},
	}
	Expect(OrgLocations(recs)).To(Equal([]string{"Chester, England, United Kingdom", "United Kingdom", ""}))
	Expect(PersonLocations(recs)).To(Equal([]string{"Chester, England, United Kingdom", "United Kingdom", ""}))
}
