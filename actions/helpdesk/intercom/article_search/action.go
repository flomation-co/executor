package helpdesk_intercom_article_search

import (
	"fmt"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Search Articles"
	Description  = "Search your Intercom Help Center articles by phrase. Filter by state (published or draft) and optionally highlight the matching terms in the results. Just-published or freshly edited articles can take a few minutes to become searchable — fetch by ID for instant results."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+magnifying-glass"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Intercom access token (Developer Hub → Authentication)", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "US (default)", Value: "us"},
			{Name: "Europe", Value: "eu"},
			{Name: "Australia", Value: "au"},
		},
	},
	{Name: "phrase", Type: core.ConnectionTypeString, Label: "Search Phrase", Placeholder: `What to search for, e.g. "getting started"`, Required: true},
	{
		Name:  "state",
		Type:  core.ConnectionTypeString,
		Label: "State",
		Options: []core.ConnectionOption{
			{Name: "Published", Value: "published"},
			{Name: "Draft", Value: "draft"},
			{Name: "All", Value: "all"},
		},
	},
	{Name: "highlight", Type: core.ConnectionTypeBoolean, Label: "Highlight Matches", Placeholder: "Tick to wrap the matching terms in highlight tags in the results"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max results (default 50)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Tick to fetch every match, page by page (ignores Limit)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Articles"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	phrase, err := intercom.RequiredString("phrase", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	q.Set("phrase", phrase)
	// "all" (and unset) means no state filter — Intercom then searches both
	// published and draft articles.
	if state := intercom.OptionalString("state", inputs); state != "" && state != "all" {
		q.Set("state", state)
	}
	if v, set := intercom.OptionalBoolSet("highlight", inputs); set && v {
		q.Set("highlight", "true")
	}

	limit, limitSet := intercom.OptionalInt("limit", inputs)
	returnAll, _ := intercom.OptionalBoolSet("return_all", inputs)
	pageSize := intercom.ClampLimit(limit, limitSet)
	if returnAll {
		pageSize = intercom.MaxPageLimit
	}
	q.Set("per_page", strconv.Itoa(pageSize))

	// Article search is Intercom's one GET-based search, and the only list
	// whose items nest a level down (data.articles rather than a top-level
	// array), so the shared ListAll can't extract it — paginate locally with
	// the same starting_after cursor loop and page-count cap.
	all := []interface{}{}
	pages := 0
	for {
		raw, err := intercom.GetObject(auth, "/articles/search", q)
		if err != nil {
			return intercom.ErrorResult(err.Error()), nil
		}
		items, cursor := searchPage(raw)
		all = append(all, items...)
		pages++
		if !returnAll || cursor == "" || len(items) == 0 || pages >= intercom.MaxAllPages {
			break
		}
		q.Set("starting_after", cursor)
	}
	return intercom.ListResult(all, fmt.Sprintf("Found %d article(s) matching %q", len(all), phrase)), nil
}

// searchPage pulls the article array and next-page cursor out of a search
// response ({"data": {"articles": [...]}, "pages": {"next": {"starting_after":
// "..."}}}). A missing array or cursor yields an empty page, which ends the
// pagination loop cleanly.
func searchPage(raw map[string]interface{}) ([]interface{}, string) {
	items := []interface{}{}
	if data, ok := raw["data"].(map[string]interface{}); ok {
		if arr, ok := data["articles"].([]interface{}); ok {
			items = arr
		}
	}
	cursor := ""
	if pages, ok := raw["pages"].(map[string]interface{}); ok {
		if next, ok := pages["next"].(map[string]interface{}); ok {
			cursor, _ = next["starting_after"].(string)
		}
	}
	return items, cursor
}
