package crm_salesforce_list_view_get_all

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many List Views"
	Description  = "List the saved views your Salesforce administrator has already built on a record type — 'My Open Opportunities', 'Hot Leads This Week'. Use the ID of the one you want with Run List View."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Opportunity, Lead, Account, Invoice__c", Required: true},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Maximum Results", Placeholder: "50 (max 2000)"},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Skip First", Placeholder: "0 — skip this many before returning results"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "List Views"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Returned"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// listViewsResponse is Salesforce's list-views envelope. Note that `size` is
// the number of views in THIS response rather than a grand total — Salesforce
// signals more with nextRecordsUrl, not with a count.
type listViewsResponse struct {
	Done           bool                     `json:"done"`
	ListViews      []map[string]interface{} `json:"listviews"`
	NextRecordsURL string                   `json:"nextRecordsUrl"`
	Size           int                      `json:"size"`
	SObjectType    string                   `json:"sobjectType"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	rawObject, err := salesforce.RequiredString("object", inputs)
	if err != nil {
		return nil, err
	}
	// The object name lands in the request path, so it is whitelist-validated
	// for the same reason a SOQL identifier is: it cannot be quoted or escaped,
	// only checked.
	object, err := salesforce.ValidateSOQLObjectName(rawObject)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	limit, set := salesforce.OptionalInt("limit", inputs)
	q.Set("limit", strconv.Itoa(salesforce.ClampLimit(limit, set)))
	if offset, ok := salesforce.OptionalInt("offset", inputs); ok && offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}

	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects/"+object+"/listviews?"+q.Encode(), nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	var payload listViewsResponse
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to parse Salesforce list views response: %s", err)), nil
	}

	views := payload.ListViews
	if views == nil {
		views = []map[string]interface{}{}
	}

	out := salesforce.ListResult(views, payload.NextRecordsURL, payload.Size, "")
	out["tool_result"] = summarise(views, object, payload.NextRecordsURL)
	return out, nil
}

// summarise names the first few views found. An operator running this action is
// almost always hunting for the ID of one particular view, so putting the
// labels in the execution log saves them opening the raw output at all.
func summarise(views []map[string]interface{}, object, nextURL string) string {
	if len(views) == 0 {
		return fmt.Sprintf("No saved list views on %s that this Salesforce user can see — list views are shared per user, group or role, so check who the connection is signed in as", object)
	}
	base := fmt.Sprintf("Found %d saved list view(s) on %s", len(views), object)
	if labels := firstLabels(views, 5); labels != "" {
		base += ": " + labels
	}
	if nextURL != "" {
		base += " — more are available, raise Maximum Results or use Skip First"
	}
	return base
}

// firstLabels renders up to max view names for the summary line.
func firstLabels(views []map[string]interface{}, max int) string {
	labels := make([]string, 0, max)
	for _, v := range views {
		label, _ := v["label"].(string)
		if strings.TrimSpace(label) == "" {
			continue
		}
		labels = append(labels, label)
		if len(labels) == max {
			break
		}
	}
	if len(labels) == 0 {
		return ""
	}
	joined := strings.Join(labels, ", ")
	if len(views) > len(labels) {
		joined += ", …"
	}
	return joined
}
