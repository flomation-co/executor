// Package report_fields searches the Google Ads field catalogue.
package report_fields

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Report: List Fields"
	Description  = "Look up which Google Ads fields exist and whether they can be selected, filtered or sorted."
	Summary      = "Discover available GAQL fields"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+book"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

// selectable_with is the field that makes this worth having: Google rejects
// combinations of resources and segments that cannot be joined, and this is the
// only way to know in advance which those are rather than discovering them one
// failed query at a time.
const fieldColumns = "name, category, data_type, selectable, filterable, sortable, is_repeated, type_url, enum_values"

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "contains", Type: core.ConnectionTypeString, Label: "Find fields whose name contains", Placeholder: "conversion"},
	{Name: "resource", Type: core.ConnectionTypeString, Label: "Or list every field on one resource", Placeholder: "campaign"},
	{Name: "selectable_only", Type: core.ConnectionTypeBoolean, Label: "Only fields that can be selected"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Maximum fields to return", Placeholder: "200"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "rows", Type: core.ConnectionTypeObject, Label: "Fields"},
	{Name: "table", Type: core.ConnectionTypeObject, Label: "Fields (flattened)"},
	{Name: "field_names", Type: core.ConnectionTypeObject, Label: "Field Names"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Query Used"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	// The field catalogue describes the API itself rather than an account, so
	// unlike every other action here it needs no customer id.
	token, err := gads.RequiredString("credential", inputs)
	if err != nil {
		return gads.ErrorResult("connect a Google Ads account"), nil
	}

	var conditions []string
	if resource := gads.OptionalString("resource", inputs); resource != "" {
		conditions = append(conditions, "name LIKE "+gads.QuoteGAQL(resource+".%"))
	}
	if contains := gads.OptionalString("contains", inputs); contains != "" {
		conditions = append(conditions, "name LIKE "+gads.QuoteGAQL("%"+contains+"%"))
	}
	if len(conditions) == 0 {
		return gads.ErrorResult("give either a resource (for example campaign) or some text to search for — the full catalogue runs to tens of thousands of fields"), nil
	}
	if gads.OptionalBool("selectable_only", inputs) {
		conditions = append(conditions, "selectable = true")
	}

	limit := gads.OptionalInt("limit", inputs)
	if limit == nil {
		fallback := int64(200)
		limit = &fallback
	}

	// The field catalogue has its own small query dialect: no FROM clause, and
	// ORDER BY is not supported on it.
	query := fmt.Sprintf("SELECT %s WHERE %s LIMIT %d", fieldColumns, strings.Join(conditions, " AND "), *limit)

	rows, err := gads.NewClient(token, "").SearchFields(flow, query)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	names := make([]string, 0, len(rows))
	for _, row := range rows {
		if name, ok := row["name"].(string); ok {
			names = append(names, name)
		}
	}

	return gads.RowsResult(rows, fmt.Sprintf("Found %d field(s)", len(rows)), map[string]interface{}{
		"query":       query,
		"field_names": names,
	}), nil
}
