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
	{Name: "person_locations", Type: core.ConnectionTypeString, Label: "Person Locations", Placeholder: "City, region or country, e.g. Chester, United Kingdom (comma-separated)"},
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
	addList(q, "person_locations[]", apollo_common.StringList("person_locations", inputs))
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
	return apollo_common.ListResult(people, fmt.Sprintf("Found %d people", len(people))), nil
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
