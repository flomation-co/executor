package helpdesk_zendesk_ticket_get_all

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Get Many Tickets"
	Description  = "List Zendesk tickets. Regular tickets are fetched via the Search API (filter by status, group, or a raw query); Suspended lists the suspended queue. Enable Return All to auto-paginate."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk+list"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var regularOnly = &core.VisibleWhen{Field: "ticket_type", Values: []string{"regular", ""}}

var Inputs = [...]core.Connection{
	{Name: "subdomain", Type: core.ConnectionTypeString, Label: "Subdomain", Placeholder: "mycompany (from mycompany.zendesk.com)", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Agent Email", Placeholder: "you@company.com (paired with the API token)"},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Zendesk API token"},
	{Name: "oauth_token", Type: core.ConnectionTypeSecret, Label: "OAuth Access Token", Placeholder: "Optional — a bearer token used instead of the email + API token"},
	{
		Name:  "ticket_type",
		Type:  core.ConnectionTypeString,
		Label: "Ticket Type",
		Options: []core.ConnectionOption{
			{Name: "Regular", Value: "regular"},
			{Name: "Suspended", Value: "suspended"},
		},
	},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (auto-paginate every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "100 per page (max 100); Return All still fetches every match"},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Query", Placeholder: `Extra Zendesk search terms, e.g. priority:high created>2024-01-01`, Visible: regularOnly},
	{
		Name:    "status",
		Type:    core.ConnectionTypeString,
		Label:   "Status",
		Visible: regularOnly,
		Options: []core.ConnectionOption{
			{Name: "New", Value: "new"},
			{Name: "Open", Value: "open"},
			{Name: "Pending", Value: "pending"},
			{Name: "On-Hold", Value: "hold"},
			{Name: "Solved", Value: "solved"},
			{Name: "Closed", Value: "closed"},
		},
	},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group", Placeholder: "Restrict to a group (ID)", Visible: regularOnly},
	{
		Name:    "sort_by",
		Type:    core.ConnectionTypeString,
		Label:   "Sort By",
		Visible: regularOnly,
		Options: []core.ConnectionOption{
			{Name: "Created At", Value: "created_at"},
			{Name: "Priority", Value: "priority"},
			{Name: "Status", Value: "status"},
			{Name: "Ticket Type", Value: "ticket_type"},
			{Name: "Updated At", Value: "updated_at"},
		},
	},
	{
		Name:    "sort_order",
		Type:    core.ConnectionTypeString,
		Label:   "Sort Order",
		Visible: regularOnly,
		Options: []core.ConnectionOption{
			{Name: "Ascending", Value: "asc"},
			{Name: "Descending", Value: "desc"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Tickets"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "next_page", Type: core.ConnectionTypeString, Label: "Next Page URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	subdomain, auth, err := zendesk.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	returnAll := zendesk.OptionalBool("return_all", inputs)
	limit, limitSet := zendesk.OptionalInt("limit", inputs)
	q := url.Values{}
	q.Set("per_page", strconv.Itoa(zendesk.ClampLimit(limit, limitSet)))

	var path, property string
	if zendesk.OptionalString("ticket_type", inputs) == "suspended" {
		path, property = "/suspended_tickets.json", "suspended_tickets"
	} else {
		// Regular tickets are listed through the Search API, whose query always
		// pins the record type to tickets, then layers optional filters.
		path, property = "/search.json", "results"
		query := "type:ticket"
		if status := zendesk.OptionalString("status", inputs); status != "" {
			query += " status:" + status
		}
		if group := zendesk.OptionalString("group_id", inputs); group != "" {
			query += " group:" + group
		}
		if extra := zendesk.OptionalString("query", inputs); extra != "" {
			query += " " + extra
		}
		q.Set("query", strings.TrimSpace(query))
		zendesk.AddFilter(q, inputs, "sort_by", "sort_by")
		zendesk.AddFilter(q, inputs, "sort_order", "sort_order")
	}

	items, next, lastRaw, pages, err := zendesk.ListResources(subdomain, auth, path, property, q, returnAll)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	out := zendesk.ListResult(items, next, lastRaw, "")
	if returnAll && next != "" && pages >= zendesk.MaxAllPages {
		out["tool_result"] = fmt.Sprintf("Fetched %d ticket(s) across %d page(s); stopped at the %d-page safety cap — pass the returned next page URL to continue", len(items), pages, zendesk.MaxAllPages)
	} else if returnAll {
		out["tool_result"] = fmt.Sprintf("Fetched all %d ticket(s) across %d page(s)", len(items), pages)
	} else {
		out["tool_result"] = fmt.Sprintf("Found %d ticket(s)", len(items))
	}
	return out, nil
}
