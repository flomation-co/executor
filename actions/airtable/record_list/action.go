package airtable_record_list

import (
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
	airtable "flomation.app/automate/executor/actions/airtable"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Record: List"
	Description  = "List or search records in an Airtable table. Filter with a formula, sort, restrict to a view, project fields, and page through results (or return all). Returns matching records."
	Website      = "https://www.flomation.co"
	Icon         = "airtable+magnifying-glass"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Airtable Personal Access Token", Placeholder: "pat...", Required: true},
	{Name: "base_id", Type: core.ConnectionTypeString, Label: "Base ID", Placeholder: "appXXXXXXXXXXXXXX", Required: true},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table (ID or name)", Placeholder: "tblXXXXXXXXXXXXXX or Table 1", Required: true},
	{Name: "filter_by_formula", Type: core.ConnectionTypeString, Label: "Filter by Formula", Placeholder: "NOT({Name} = 'Admin')"},
	{Name: "view", Type: core.ConnectionTypeString, Label: "View", Placeholder: "View name or ID (optional)"},
	{Name: "return_fields", Type: core.ConnectionTypeString, Label: "Return Fields", Placeholder: "Comma-separated field names to return (optional)"},
	{Name: "sort_field", Type: core.ConnectionTypeString, Label: "Sort Field", Placeholder: "Field name to sort by (optional)"},
	{Name: "sort_direction", Type: core.ConnectionTypeString, Label: "Sort Direction", Options: []core.ConnectionOption{
		{Name: "Ascending", Value: "asc"},
		{Name: "Descending", Value: "desc"},
	}},
	{Name: "sort", Type: core.ConnectionTypeObject, Label: "Sort (JSON, advanced)", Placeholder: `[{"field":"Created","direction":"desc"}] (overrides Sort Field)`},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Page through every matching record (up to a safety cap)"},
	{Name: "max_records", Type: core.ConnectionTypeInteger, Label: "Max Records", Placeholder: "Cap on total records returned (optional)"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "Records per page, max 100 (default 100)"},
	{Name: "offset", Type: core.ConnectionTypeString, Label: "Offset", Placeholder: "Pagination offset from a previous page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "records", Type: core.ConnectionTypeObject, Label: "Records"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "offset", Type: core.ConnectionTypeString, Label: "Next Offset"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := airtable.GetAccessToken(inputs)
	if err != nil {
		return nil, err
	}
	baseID, err := airtable.RequiredString("base_id", inputs)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}
	table, err := airtable.RequiredString("table", inputs)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}

	q, err := airtable.BuildListQuery(inputs)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}

	pageSize := 100
	if ps, ok := airtable.OptionalInt("page_size", inputs); ok && ps > 0 {
		if ps > 100 {
			ps = 100 // Airtable's documented per-page maximum
		}
		pageSize = ps
	}
	q.Set("pageSize", strconv.Itoa(pageSize))
	if mr, ok := airtable.OptionalInt("max_records", inputs); ok && mr > 0 {
		q.Set("maxRecords", strconv.Itoa(mr))
	}

	if airtable.OptionalBool("return_all", inputs) {
		var all []interface{}
		var lastRaw map[string]interface{}
		offset := airtable.OptionalString("offset", inputs)
		pages := 0
		for {
			if offset != "" {
				q.Set("offset", offset)
			} else {
				q.Del("offset")
			}
			recs, next, raw, err := airtable.ListRecordsPage(token, baseID, table, q)
			if err != nil {
				return airtable.ErrorResult(err.Error()), nil
			}
			all = append(all, recs...)
			lastRaw = raw
			offset = next
			pages++
			if offset == "" || pages >= airtable.MaxAllPages {
				break
			}
		}
		out := airtable.ListResult(all, offset, lastRaw, "")
		if offset != "" {
			out["tool_result"] = fmt.Sprintf("Fetched %d record(s) across %d page(s); stopped at the %d-page safety cap — pass the returned offset to continue", len(all), pages, airtable.MaxAllPages)
		} else {
			out["tool_result"] = fmt.Sprintf("Fetched all %d record(s) across %d page(s)", len(all), pages)
		}
		return out, nil
	}

	if offset := airtable.OptionalString("offset", inputs); offset != "" {
		q.Set("offset", offset)
	}
	recs, next, raw, err := airtable.ListRecordsPage(token, baseID, table, q)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}
	return airtable.ListResult(recs, next, raw, fmt.Sprintf("Found %d record(s)", len(recs))), nil
}
