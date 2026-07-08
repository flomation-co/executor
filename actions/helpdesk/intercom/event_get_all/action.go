package helpdesk_intercom_event_get_all

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Get Many Events"
	Description  = "List the events recorded for one contact, newest first. Intercom keeps events queryable for around 90 days. Enable Summary Only for one row per event type with counts."
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
	{
		Name:  "select_by",
		Type:  core.ConnectionTypeString,
		Label: "Find Contact By",
		Options: []core.ConnectionOption{
			{Name: "Intercom ID", Value: "intercom_user_id"},
			{Name: "External ID", Value: "user_id"},
			{Name: "Email", Value: "email"},
		},
	},
	{Name: "value", Type: core.ConnectionTypeString, Label: "Value", Placeholder: "The contact's Intercom ID, external ID, or email — whichever matches Find Contact By", Required: true},
	{Name: "summary", Type: core.ConnectionTypeBoolean, Label: "Summary Only", Placeholder: "One row per event type with counts, instead of each individual event"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max results (default 50)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Fetch every event (ignores Limit)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Events"},
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

	value, err := intercom.RequiredString("value", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	selectBy := intercom.OptionalString("select_by", inputs)
	if selectBy == "" {
		selectBy = "intercom_user_id"
	}
	switch selectBy {
	case "intercom_user_id", "user_id", "email":
	default:
		return intercom.ErrorResult("Find Contact By must be Intercom ID, External ID, or Email"), nil
	}

	q := url.Values{}
	q.Set("type", "user")
	q.Set("filter["+selectBy+"]", value)
	if v, set := intercom.OptionalBoolSet("summary", inputs); set && v {
		q.Set("summary", "true")
	}

	limit, limitSet := intercom.OptionalInt("limit", inputs)
	returnAll, _ := intercom.OptionalBoolSet("return_all", inputs)
	items, err := listEvents(auth, q, intercom.ClampLimit(limit, limitSet), returnAll)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ListResult(items, fmt.Sprintf("Retrieved %d event(s)", len(items))), nil
}

// listEvents pages through GET /events. Unlike the cursor lists common.go
// paginates, the events envelope's pages.next is a full URL string (legacy
// pagination) rather than a {starting_after} object, so this local helper
// follows it — reusing only the query string of the next link against the
// fixed /events path, never a response-supplied host.
func listEvents(auth intercom.Auth, query url.Values, pageSize int, returnAll bool) ([]interface{}, error) {
	if returnAll {
		pageSize = intercom.MaxPageLimit
	}
	query.Set("per_page", strconv.Itoa(pageSize))
	all := []interface{}{}
	pages := 0
	for {
		resp, err := intercom.Do(auth, http.MethodGet, "/events", nil, query)
		if err != nil {
			return nil, err
		}
		if err := intercom.CheckResponse(resp); err != nil {
			return nil, err
		}
		raw, err := intercom.DecodeBody(resp)
		if err != nil {
			return nil, err
		}
		items, _ := raw["events"].([]interface{})
		all = append(all, items...)
		pages++
		next := nextEventsQuery(raw, query)
		if !returnAll || next == nil || len(items) == 0 || pages >= intercom.MaxAllPages {
			break
		}
		query = next
	}
	return all, nil
}

// nextEventsQuery extracts the next-page query from an events envelope.
// pages.next is normally a URL string whose query string carries the
// continuation params; an object-form {starting_after} cursor is also handled
// in case the endpoint ever migrates to match the other lists. nil means "no
// more pages".
func nextEventsQuery(raw map[string]interface{}, current url.Values) url.Values {
	pagesObj, ok := raw["pages"].(map[string]interface{})
	if !ok {
		return nil
	}
	switch next := pagesObj["next"].(type) {
	case string:
		u, err := url.Parse(next)
		if err != nil || u.RawQuery == "" {
			return nil
		}
		q, err := url.ParseQuery(u.RawQuery)
		if err != nil || len(q) == 0 {
			return nil
		}
		return q
	case map[string]interface{}:
		cursor, _ := next["starting_after"].(string)
		if cursor == "" {
			return nil
		}
		q := url.Values{}
		for k, vs := range current {
			q[k] = vs
		}
		q.Set("starting_after", cursor)
		return q
	}
	return nil
}
