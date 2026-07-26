package crm_salesforce_list_view_run

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Run List View"
	Description  = "Run one of the saved views your Salesforce administrator has already built and get its records back. No query writing at all — and because it uses the org's own view, the results stay right when the admin changes the filters."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clipboard-list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Opportunity, Lead, Account, Invoice__c", Required: true},
	{Name: "list_view_id", Type: core.ConnectionTypeString, Label: "List View", Placeholder: "00B5f000004XyzAEAS — from Get Many List Views", Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return Every Page", Placeholder: "Keep fetching until every record in the view has been returned"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Maximum Results", Placeholder: "50 per page (max 2000)"},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Skip First", Placeholder: "0 — skip this many before returning results"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Records"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Returned"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// listViewResults is Salesforce's list-view results envelope.
//
// The shape is unlike every other read in this node: records are NOT field maps
// but arrays of {fieldNameOrPath, value} cells, mirroring the columns of the
// view as it appears on screen. Handed straight to a flow that would be
// unusable — nothing downstream could reference ${node.results[0].Email} — so
// Execute flattens each record back into an ordinary field map.
type listViewResults struct {
	Columns        []listViewColumn `json:"columns"`
	DeveloperName  string           `json:"developerName"`
	Done           bool             `json:"done"`
	ID             string           `json:"id"`
	Label          string           `json:"label"`
	NextRecordsURL string           `json:"nextRecordsUrl"`
	Records        []listViewRecord `json:"records"`
	Size           int              `json:"size"`
}

type listViewColumn struct {
	FieldNameOrPath string `json:"fieldNameOrPath"`
	Label           string `json:"label"`
	Hidden          bool   `json:"hidden"`
}

type listViewRecord struct {
	Columns []listViewCell `json:"columns"`
}

type listViewCell struct {
	FieldNameOrPath string      `json:"fieldNameOrPath"`
	Value           interface{} `json:"value"`
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
	// Both the object name and the view ID land in the request path, so both are
	// validated here rather than handed to Salesforce to reject.
	object, err := salesforce.ValidateSOQLObjectName(rawObject)
	if err != nil {
		return nil, err
	}
	listViewID, err := salesforce.RequiredString("list_view_id", inputs)
	if err != nil {
		return nil, err
	}
	if err := salesforce.ValidateRecordID(listViewID); err != nil {
		return nil, fmt.Errorf("%w — a list view ID begins 00B; run Get Many List Views to find the one you want", err)
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)

	q := url.Values{}
	limit, set := salesforce.OptionalInt("limit", inputs)
	q.Set("limit", strconv.Itoa(salesforce.ClampLimit(limit, set)))
	if offset, ok := salesforce.OptionalInt("offset", inputs); ok && offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}

	path := "/sobjects/" + object + "/listviews/" + url.PathEscape(listViewID) + "/results?" + q.Encode()

	var (
		records []map[string]interface{}
		payload listViewResults
		nextURL string
		pages   int
		// absolute tracks whether the next request is against a path Salesforce
		// handed back (which already carries its own /services/data/vNN prefix)
		// rather than one built below the version root.
		absolute bool
	)

	for {
		var resp *salesforce.APIResponse
		if absolute {
			resp, err = salesforce.ExecuteAbsolute(instanceURL, token, http.MethodGet, path, nil)
		} else {
			resp, err = salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, path, nil)
		}
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		if err := salesforce.CheckResponse(resp); err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}

		payload = listViewResults{}
		if err := json.Unmarshal(resp.Body, &payload); err != nil {
			return salesforce.ErrorResult(fmt.Sprintf("failed to parse Salesforce list view response: %s", err)), nil
		}
		pages++
		records = append(records, flatten(payload.Records)...)
		nextURL = payload.NextRecordsURL

		// The same bound the shared query helper applies: a view over a large
		// object must never spin unlimited requests against an API allowance
		// the customer's other integrations are drawing on too.
		if payload.Done || nextURL == "" || !returnAll || pages >= salesforce.MaxAllPages {
			break
		}
		path = nextURL
		absolute = true
	}

	// Salesforce's `size` counts the records in the LAST response, not the view
	// as a whole — there is no grand total on this endpoint — so the honest
	// figure to report across a paged run is what was actually collected.
	out := salesforce.ListResult(records, nextURL, len(records), "")
	out["tool_result"] = summarise(len(records), payload, pages, nextURL, returnAll)
	return out, nil
}

// flatten turns Salesforce's column-per-cell records into ordinary field maps,
// so a record reads as {"Name":"Acme Ltd","Email":"..."} exactly like every
// other Salesforce read in this node.
//
// fieldNameOrPath is used as the key rather than the column label: labels are
// translated per user language and renamed by admins, so a flow keyed on them
// would break the day someone tidies up the view. Related fields keep their
// dotted path (Account.Name), which is what a SOQL read of the same field
// returns too.
func flatten(records []listViewRecord) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(records))
	for _, rec := range records {
		flat := make(map[string]interface{}, len(rec.Columns))
		for _, cell := range rec.Columns {
			if cell.FieldNameOrPath == "" {
				continue
			}
			flat[cell.FieldNameOrPath] = cell.Value
		}
		out = append(out, flat)
	}
	return out
}

// summarise reports the run in the view's own terms — the operator picked
// "My Open Opportunities" from a dropdown and that is what they want to read
// back, not an 18-character ID.
func summarise(count int, payload listViewResults, pages int, nextURL string, returnAll bool) string {
	name := payload.Label
	if name == "" {
		name = payload.DeveloperName
	}
	if name == "" {
		name = "the list view"
	} else {
		name = fmt.Sprintf("%q", name)
	}

	switch {
	case count == 0:
		return fmt.Sprintf("%s returned no records — the view's own filters decide what is in it, so check it in Salesforce if you expected some", name)
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		return fmt.Sprintf("Returned %d record(s) from %s across %d page(s), then stopped at the %d-page safety limit", count, name, pages, salesforce.MaxAllPages)
	case returnAll:
		return fmt.Sprintf("Returned all %d record(s) from %s across %d page(s)", count, name, pages)
	case nextURL != "":
		return fmt.Sprintf("Returned %d record(s) from %s — turn on Return Every Page to fetch the rest", count, name)
	default:
		return fmt.Sprintf("Returned %d record(s) from %s", count, name)
	}
}
