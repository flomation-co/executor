package people_search

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "People: Search"
	Description  = "Search Apollo's people database by title, seniority, location and keywords."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+magnifying-glass"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "q_keywords", Type: core.ConnectionTypeString, Label: "Keywords", Placeholder: "Free-text search across name, title, company"},
	{Name: "person_titles", Type: core.ConnectionTypeString, Label: "Job Titles", Placeholder: "CEO, Head of Sales (comma-separated)"},
	{Name: "person_seniorities", Type: core.ConnectionTypeString, Label: "Seniorities", Placeholder: "owner, founder, c_suite, vp, director (comma-separated)"},
	{Name: "person_locations", Type: core.ConnectionTypeString, Label: "Person Locations (where the PERSON lives)", Placeholder: "Chester, United Kingdom — use a CITY (UK counties are often unknown to Apollo and get ignored); separate multiple with ;"},
	{Name: "organization_locations", Type: core.ConnectionTypeString, Label: "Company HQ Locations (where the COMPANY is)", Placeholder: "Chester, United Kingdom — separate MULTIPLE locations with ;"},
	{Name: "contact_email_status", Type: core.ConnectionTypeString, Label: "Email Status", Options: []core.ConnectionOption{{Name: "Verified only", Value: "verified"}, {Name: "Verified or likely", Value: "verified,likely_to_engage"}, {Name: "Any", Value: ""}}},
	{Name: "organization_domains", Type: core.ConnectionTypeString, Label: "Company Domains", Placeholder: "example.com (comma-separated)"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1"},
	{Name: "per_page", Type: core.ConnectionTypeInteger, Label: "Per Page", Placeholder: "25 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "People"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	// CRITICAL: Apollo's people search reads EVERY filter from the URL query
	// string, not the JSON body — array filters use bracket notation
	// (person_locations[]=..., q_organization_domains_list[]=...). Sending them
	// in the body (as this action originally did) meant Apollo silently ignored
	// the location, domain, title and seniority filters and returned a generic
	// relevance-ranked list, so results were never actually scoped. See
	// https://docs.apollo.io/reference/people-api-search.
	q := url.Values{}
	if v := strings.TrimSpace(apollo_common.OptionalString("q_keywords", inputs)); v != "" {
		q.Set("q_keywords", v)
	}
	addList(q, "person_titles[]", apollo_common.StringList("person_titles", inputs))
	addList(q, "person_seniorities[]", apollo_common.StringList("person_seniorities", inputs))
	// Locations are semicolon-separated — see apollo_common.LocationList. A comma
	// is part of one Apollo location value, so splitting on it ORs a city with
	// its country and quietly widens the search to the whole country.
	//
	// person_locations and organization_locations are deliberately separate
	// inputs: the first is where the PERSON is, the second where their EMPLOYER
	// is. They are not interchangeable — a "verified local" standard is a claim
	// about the person, and a company HQ says nothing about where a given
	// employee actually sits.
	addList(q, "person_locations[]", apollo_common.LocationList("person_locations", inputs))
	addList(q, "organization_locations[]", apollo_common.LocationList("organization_locations", inputs))
	addList(q, "contact_email_status[]", apollo_common.StringList("contact_email_status", inputs))
	addList(q, "q_organization_domains_list[]", normaliseDomains(apollo_common.StringList("organization_domains", inputs)))
	if p := apollo_common.OptionalInt("page", inputs); p != nil {
		q.Set("page", strconv.FormatInt(*p, 10))
	}
	if pp := apollo_common.OptionalInt("per_page", inputs); pp != nil {
		q.Set("per_page", strconv.FormatInt(*pp, 10))
	}

	// The filters live in the query string; the body is an empty JSON object so
	// the request still carries a Content-Type: application/json header, matching
	// Apollo's documented request shape.
	resp, err := apollo_common.NewClient(apiKey).Request(flow, http.MethodPost, "/mixed_people/api_search", q, map[string]interface{}{})
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	people := apollo_common.Arr(resp, "people")
	// Warn loudly if the plan gated the personal data (obfuscated surnames, no
	// emails/cities) so a caller cannot mistake masked people for real contacts.
	summary := apollo_common.GatePrefixSearch(fmt.Sprintf("Found %d people", len(people)), people)

	// Apollo drops location values outside its taxonomy and answers an
	// unfiltered search instead, so check the results against what was asked for.
	//
	// Person location is the right thing to check — person_locations filters on
	// the individual — but on a plan that masks personal data every person's city
	// is null. In that case the honest answer is "unknown", so fall back to the
	// employer's location (which is never masked) and label it as the
	// company-level signal it is, rather than either staying silent or claiming
	// the filter failed.
	requestedPersonLocations := apollo_common.LocationList("person_locations", inputs)
	personLocations := apollo_common.PersonLocations(people)
	orgLocations := apollo_common.PeopleOrgLocations(people)

	if warn := apollo_common.LocationIgnoredWarning(requestedPersonLocations, personLocations); warn != "" {
		summary = warn + "\n" + summary
	} else if note := apollo_common.PersonLocationNote(requestedPersonLocations, personLocations, orgLocations); note != "" {
		summary = note + "\n" + summary
	}
	// Then state each person's own location separately from their employer's, so
	// a company HQ is never read as confirming where the individual is.
	if prov := apollo_common.PeopleProvenance(people); prov != "" {
		summary += "\n" + prov
	}
	return apollo_common.ListResult(people, summary), nil
}

// addList appends each non-empty value under the bracketed key so Apollo sees a
// repeated query parameter (key[]=a&key[]=b).
func addList(q url.Values, key string, values []string) {
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			q.Add(key, t)
		}
	}
}

// normaliseDomains reduces each entry to a bare registrable domain, since Apollo
// matches q_organization_domains_list on exact bare domains and will silently
// drop anything with a scheme, www., @, a path or surrounding whitespace. A
// blank/invalid entry is dropped so it can never widen the search.
func normaliseDomains(raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, d := range raw {
		d = strings.ToLower(strings.TrimSpace(d))
		if i := strings.Index(d, "://"); i != -1 {
			d = d[i+3:]
		}
		d = strings.TrimPrefix(d, "@")
		if i := strings.IndexByte(d, '@'); i != -1 {
			d = d[i+1:]
		}
		if i := strings.IndexByte(d, '/'); i != -1 {
			d = d[:i]
		}
		d = strings.TrimPrefix(d, "www.")
		d = strings.Trim(d, ".")
		if d != "" && strings.Contains(d, ".") {
			out = append(out, d)
		}
	}
	return out
}
