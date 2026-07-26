// Package crm_salesforce_object_get_all lists every object available in the
// connected org — standard and custom.
//
// Salesforce orgs are not interchangeable: Quotes, Orders and Products are
// feature-gated per edition, every org has its own custom objects, and the list
// is filtered by the connected user's permissions. Reading the org's real
// configuration instead of assuming a fixed list is what lets a flow adapt, and
// it is what stops a dropdown elsewhere in the node offering an object that
// could never work here.
//
// The filters exist because the raw list is not a useful thing to hand a
// non-technical operator: a stock org returns roughly a thousand objects, most
// of them internal plumbing (ApexLog, FieldPermissions, the *Share and *History
// shadows) that nobody automates against.
package crm_salesforce_object_get_all

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: List Objects"
	Description  = "List every object in your Salesforce org — the standard ones and your own custom ones — so a flow can work with whatever this org is actually set up for."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "custom_only", Type: core.ConnectionTypeBoolean, Label: "Custom Objects Only"},
	{Name: "createable_only", Type: core.ConnectionTypeBoolean, Label: "Only Objects You Can Add Records To"},
	{Name: "filter", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "invoice — matches the object's name or its label"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Objects"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Available"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// globalDescribe is the /sobjects envelope. Only sobjects is read; the encoding
// and maxBatchSize siblings say nothing an operator needs.
type globalDescribe struct {
	SObjects []map[string]interface{} `json:"sobjects"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects", nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	var gd globalDescribe
	if err := json.Unmarshal(resp.Body, &gd); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("could not read the object list from Salesforce: %v", err)), nil
	}
	total := len(gd.SObjects)

	customOnly := salesforce.OptionalBool("custom_only", inputs)
	createableOnly := salesforce.OptionalBool("createable_only", inputs)
	search := strings.ToLower(salesforce.OptionalString("filter", inputs))

	objects := make([]map[string]interface{}, 0, total)
	for _, obj := range gd.SObjects {
		if customOnly && !boolField(obj, "custom") {
			continue
		}
		if createableOnly && !boolField(obj, "createable") {
			continue
		}
		if search != "" && !matches(obj, search) {
			continue
		}
		objects = append(objects, obj)
	}

	// Sort by the human label, then the API name as a tie-break. Salesforce
	// returns these in an order of its own that is neither alphabetical nor
	// stable between orgs, and an unsorted thousand-row list is unreadable.
	sort.SliceStable(objects, func(i, j int) bool {
		li, lj := strings.ToLower(stringField(objects[i], "label")), strings.ToLower(stringField(objects[j], "label"))
		if li != lj {
			return li < lj
		}
		return stringField(objects[i], "name") < stringField(objects[j], "name")
	})

	summary := fmt.Sprintf("Found %d object(s) in this Salesforce org", len(objects))
	if len(objects) != total {
		summary = fmt.Sprintf("Found %d of %d object(s) in this Salesforce org after filtering", len(objects), total)
	}
	return salesforce.ListResult(objects, "", total, summary), nil
}

// matches reports whether the search text appears in the object's API name or
// its human label — an operator looking for "invoice" should find both
// Invoice__c and an object labelled "Customer Invoice".
func matches(obj map[string]interface{}, search string) bool {
	return strings.Contains(strings.ToLower(stringField(obj, "name")), search) ||
		strings.Contains(strings.ToLower(stringField(obj, "label")), search)
}

func stringField(obj map[string]interface{}, key string) string {
	v, _ := obj[key].(string)
	return v
}

func boolField(obj map[string]interface{}, key string) bool {
	v, _ := obj[key].(bool)
	return v
}
