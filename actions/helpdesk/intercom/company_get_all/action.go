package helpdesk_intercom_company_get_all

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
	Name         = "Intercom: Get Many Companies"
	Description  = "List the companies in your Intercom workspace, optionally only those with a given tag or in a segment. Enable Return All to auto-paginate every company."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+list"
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
	{Name: "tag_id", Type: core.ConnectionTypeString, Label: "Tag", Placeholder: "Only list companies with this tag — leave empty for all"},
	{Name: "segment_id", Type: core.ConnectionTypeString, Label: "Segment", Placeholder: "Only list companies in this segment — leave empty for all"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max results (default 50)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every company (ignores Limit)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Companies"},
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
	returnAll, _ := intercom.OptionalBoolSet("return_all", inputs)
	limit, _ := intercom.OptionalInt("limit", inputs)
	tagID := intercom.OptionalString("tag_id", inputs)
	segmentID := intercom.OptionalString("segment_id", inputs)

	var items []interface{}
	if tagID != "" || segmentID != "" {
		// Tag/segment filtering only exists on the legacy filter endpoint
		// (GET /companies?tag_id=&segment_id=), which uses classic page/
		// per_page pagination rather than the starting_after cursor.
		items, err = listFiltered(auth, tagID, segmentID, limit, returnAll)
	} else {
		// The cursor-paginated company list is POST /companies/list with the
		// pagination object inside the body (like the search endpoints, but
		// with no query at all) — SearchAll owns that shape.
		items, err = intercom.SearchAll(auth, "/companies/list", nil, "data", limit, returnAll)
	}
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	noun := "companies"
	if len(items) == 1 {
		noun = "company"
	}
	return intercom.ListResult(items, fmt.Sprintf("Retrieved %d %s", len(items), noun)), nil
}

// listFiltered pages through GET /companies with tag_id/segment_id filters.
// Unlike every other Intercom list, this endpoint paginates classically —
// an incrementing page query param (default 15/page) instead of a
// starting_after cursor — so it gets its own small loop; a short page or an
// empty page ends it, with ListAll's same MaxAllPages safety cap.
func listFiltered(auth intercom.Auth, tagID, segmentID string, limit int, returnAll bool) ([]interface{}, error) {
	pageSize := intercom.ClampLimit(limit, limit > 0)
	if returnAll {
		pageSize = intercom.MaxPageLimit
	}
	q := url.Values{}
	if tagID != "" {
		q.Set("tag_id", tagID)
	}
	if segmentID != "" {
		q.Set("segment_id", segmentID)
	}
	q.Set("per_page", strconv.Itoa(pageSize))

	all := []interface{}{}
	for page := 1; page <= intercom.MaxAllPages; page++ {
		q.Set("page", strconv.Itoa(page))
		// ListPage's array extraction handles this envelope too ("data", with
		// the single-array fallback catching a legacy "companies" key); its
		// cursor is always "" here, so the loop is bounded locally.
		items, _, err := intercom.ListPage(auth, "/companies", q, "data")
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if !returnAll || len(items) < pageSize {
			break
		}
	}
	return all, nil
}
