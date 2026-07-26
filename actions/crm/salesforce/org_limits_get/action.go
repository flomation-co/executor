// Package crm_salesforce_org_limits_get reads the org's remaining allowances.
//
// Salesforce meters an org's API calls daily and cuts EVERY integration off
// when the allowance runs out — not just the one that spent it. A flow about to
// loop over a few thousand rows can check what is left first and alert or bail
// out, instead of taking the customer's whole Salesforce estate offline for the
// rest of the day. That is the difference between a noisy automation and an
// outage somebody has to explain.
package crm_salesforce_org_limits_get

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Org Limits"
	Description  = "Check how much of your Salesforce allowance is left — daily API calls, data storage, file storage — so a flow can stop or warn before it uses it all up."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+gauge"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "limit_name", Type: core.ConnectionTypeString, Label: "Just One Allowance", Placeholder: "DailyApiRequests — leave blank to return them all"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Allowance Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Limits"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// headlineLimits are the allowances worth putting in the summary line, in the
// order an operator cares about them. Salesforce returns fifty-odd entries;
// three of them are the ones that actually stop work happening.
var headlineLimits = []struct {
	key   string
	label string
	unit  string
}{
	{"DailyApiRequests", "Daily API calls", ""},
	{"DataStorageMB", "Data storage", " MB"},
	{"FileStorageMB", "File storage", " MB"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/limits", nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	// Reading limits needs "View Setup and Configuration" (and Manage Users for
	// the storage figures), which plenty of integration users are not given. A
	// bare 403 sends people hunting for a broken connection, so name the
	// permission instead — the connection itself is fine.
	if resp.StatusCode == http.StatusForbidden && forbiddenIsPermission(resp.Body) {
		return salesforce.ErrorResult("the connected Salesforce user is not allowed to read the org's limits — ask your administrator to grant it the \"View Setup and Configuration\" permission (storage figures also need \"Manage Users\")"), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	var limits map[string]interface{}
	if err := json.Unmarshal(resp.Body, &limits); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to parse the Salesforce limits response: %v", err)), nil
	}

	// One allowance asked for by name: return just that one, so an If node can
	// read result.Remaining without knowing which of fifty keys to dig into.
	if wanted := salesforce.OptionalString("limit_name", inputs); wanted != "" {
		key, entry, ok := findLimit(limits, wanted)
		if !ok {
			// A misspelled name is a configuration mistake, not a Salesforce
			// failure, so it fails hard — and listing what IS available is the
			// only useful thing to say about it.
			return nil, fmt.Errorf("%q is not one of your org's allowances — available names are: %s", wanted, strings.Join(salesforce.SortedKeys(limits), ", "))
		}
		return salesforce.RecordResult(key, entry, describeLimit(key, "", "", entry)), nil
	}

	return salesforce.RecordResult("", limits, summarise(limits)), nil
}

// forbiddenIsPermission decides whether a 403 from /limits really is the
// missing-setup-permission case.
//
// Salesforce uses 403 for two very different things on this endpoint, and on
// THIS action of all actions the difference matters: an org that has already
// burned its daily API allowance is answered 403 REQUEST_LIMIT_EXCEEDED, which
// is the exact condition the operator opened this node to watch for. Reporting
// that as "ask your administrator for View Setup and Configuration" would send
// them to the wrong person about a problem that fixes itself at midnight —
// so anything Salesforce has already explained goes to CheckResponse, which
// translates the code into plain English.
func forbiddenIsPermission(body []byte) bool {
	var errs []struct {
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(body, &errs); err != nil {
		return true
	}
	for _, e := range errs {
		switch strings.ToUpper(strings.TrimSpace(e.ErrorCode)) {
		case "REQUEST_LIMIT_EXCEEDED", "API_DISABLED_FOR_ORG", "SERVER_UNAVAILABLE", "INVALID_SESSION_ID":
			return false
		}
	}
	return true
}

// findLimit resolves an allowance by name, case-insensitively — nobody should
// have to remember whether Salesforce capitalises the "api" in DailyApiRequests.
func findLimit(limits map[string]interface{}, wanted string) (string, map[string]interface{}, bool) {
	for key, value := range limits {
		if !strings.EqualFold(key, wanted) {
			continue
		}
		entry, ok := value.(map[string]interface{})
		if !ok {
			return key, map[string]interface{}{}, true
		}
		return key, entry, true
	}
	return "", nil, false
}

// summarise renders the headline allowances into one readable line, skipping
// any the org does not report (a Developer Edition org has no file storage
// entry, and the permission the user holds decides what comes back at all).
func summarise(limits map[string]interface{}) string {
	parts := make([]string, 0, len(headlineLimits))
	for _, h := range headlineLimits {
		entry, ok := limits[h.key].(map[string]interface{})
		if !ok {
			continue
		}
		parts = append(parts, describeLimit(h.key, h.label, h.unit, entry))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Read %d Salesforce allowance(s)", len(limits))
	}
	return strings.Join(parts, "; ")
}

// describeLimit renders one allowance as "Daily API calls: 14,998 of 15,000
// left (99%)". Salesforce reports every limit as Max plus Remaining, so the
// amount USED — the number people actually ask about — has to be derived.
//
// The percentage is floored rather than rounded: rounding 99.98% up to "100%"
// tells an operator watching their allowance that nothing has been spent, and
// this line exists precisely so they can see that something has.
func describeLimit(key, label, unit string, entry map[string]interface{}) string {
	if label == "" {
		label = key
	}
	max, maxOK := numberField(entry, "Max")
	remaining, remainingOK := numberField(entry, "Remaining")
	if !maxOK || !remainingOK {
		// Some entries (per-namespace package limits) nest another map instead
		// of carrying Max/Remaining themselves; there is nothing to total up.
		return fmt.Sprintf("%s: see the result for details", label)
	}
	if max <= 0 {
		return fmt.Sprintf("%s: %s%s left", label, formatNumber(remaining), unit)
	}
	return fmt.Sprintf("%s: %s of %s%s left (%.0f%%)", label, formatNumber(remaining), formatNumber(max), unit, math.Floor(remaining/max*100))
}

// numberField reads a numeric field from a limit entry. JSON numbers decode to
// float64, but an int can arrive when the value has been round-tripped through
// another node, so both are accepted.
func numberField(entry map[string]interface{}, field string) (float64, bool) {
	switch v := entry[field].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	}
	return 0, false
}

// formatNumber renders a whole number with thousands separators. "14,998" reads
// as a number at a glance; "14998" has to be counted.
func formatNumber(v float64) string {
	digits := strconv.FormatInt(int64(v), 10)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	var b strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return sign + b.String()
}
