package helpdesk_intercom_note_get_all

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
	Name         = "Intercom: Get Many Notes"
	Description  = "List the private notes on a contact's timeline, newest first. Enable Return All to fetch every note."
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
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The Intercom contact whose notes to fetch", Required: true},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max results (default 50)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every note (ignores Limit)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Notes"},
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
	contactID, err := intercom.RequiredString("contact_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	returnAll, _ := intercom.OptionalBoolSet("return_all", inputs)
	limit, limitSet := intercom.OptionalInt("limit", inputs)
	pageSize := intercom.ClampLimit(limit, limitSet)
	if returnAll {
		pageSize = intercom.MaxPageLimit
	}
	items, err := listNotes(auth, contactID, pageSize, returnAll)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ListResult(items, fmt.Sprintf("Retrieved %d note(s) for contact %s", len(items), contactID)), nil
}

// listNotes pages through GET /contacts/{id}/notes. Notes are one of
// Intercom's classic page-numbered lists — pages.next is a URL string, not the
// {starting_after} cursor object the shared ListAll follows — so this loop
// walks the page number instead, stopping at the shared MaxAllPages cap.
func listNotes(auth intercom.Auth, contactID string, pageSize int, returnAll bool) ([]interface{}, error) {
	path := "/contacts/" + url.PathEscape(contactID) + "/notes"
	q := url.Values{}
	q.Set("per_page", strconv.Itoa(pageSize))
	all := []interface{}{}
	for page := 1; page <= intercom.MaxAllPages; page++ {
		q.Set("page", strconv.Itoa(page))
		raw, err := intercom.GetObject(auth, path, q)
		if err != nil {
			return nil, err
		}
		items, _ := raw["data"].([]interface{})
		all = append(all, items...)
		if !returnAll || len(items) == 0 || !hasMorePages(raw, page) {
			break
		}
	}
	return all, nil
}

// hasMorePages reports whether the classic pages envelope says a page follows
// the current one.
func hasMorePages(raw map[string]interface{}, page int) bool {
	pages, ok := raw["pages"].(map[string]interface{})
	if !ok {
		return false
	}
	total, ok := pages["total_pages"].(float64)
	if !ok {
		return false
	}
	return page < int(total)
}
