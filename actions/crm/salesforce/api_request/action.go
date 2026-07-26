// Package crm_salesforce_api_request makes any authenticated call to the
// Salesforce API.
//
// It is the escape hatch, and it is deliberately the last action in the group.
// No curated action list will ever cover the whole platform — Tooling,
// Metadata, UI API, Connect, Commerce, Knowledge, Analytics and every org's own
// Apex REST endpoints all hang off the same host and the same token — so this
// costs almost nothing on top of the shared client and means a customer is
// never blocked waiting for Flomation to ship an action.
//
// Two paths are reachable. A path that does not start with /services is taken
// as relative to the versioned REST root, so "/limits" becomes
// /services/data/v62.0/limits — which is what an operator copying an example
// out of Salesforce's REST guide will have. A path that DOES start with
// /services is sent as-is, which is what makes /services/apexrest/... and the
// non-versioned endpoints reachable too.
//
// The one hard rule in here is that the path must begin with a slash. It is
// appended to the org's origin, so a path starting "@evil.example" would turn
// the origin into userinfo and send the customer's access token to somebody
// else's host.
package crm_salesforce_api_request

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Custom API Call"
	Description  = "Advanced: call any part of the Salesforce API with your existing connection. For anything the other Salesforce actions do not cover yet, including your org's own Apex REST endpoints."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+bolt"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{
		Name:  "method",
		Type:  core.ConnectionTypeString,
		Label: "Method",
		Options: []core.ConnectionOption{
			{Name: "GET", Value: "GET"},
			{Name: "POST", Value: "POST"},
			{Name: "PATCH", Value: "PATCH"},
			{Name: "PUT", Value: "PUT"},
			{Name: "DELETE", Value: "DELETE"},
		},
	},
	{Name: "path", Type: core.ConnectionTypeString, Label: "Path", Placeholder: "/limits or /sobjects/Account/describe — or /services/apexrest/MyEndpoint", Required: true},
	{Name: "query", Type: core.ConnectionTypeObject, Label: "Query Parameters", Placeholder: "{\"q\":\"SELECT Id FROM Account LIMIT 5\"} — added to the path for you"},
	{Name: "body", Type: core.ConnectionTypeObject, Label: "Body", Placeholder: "{\"Name\":\"Acme Ltd\"} — for POST, PATCH and PUT", Visible: &core.VisibleWhen{Field: "method", Values: []string{"POST", "PATCH", "PUT"}}},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// allowedMethods is the closed set of HTTP verbs this action will send. The
// shared client only attaches a body for POST/PATCH/PUT, and anything outside
// this list would be an operator typo rather than a real Salesforce endpoint.
var allowedMethods = map[string]string{
	"":       http.MethodGet,
	"GET":    http.MethodGet,
	"POST":   http.MethodPost,
	"PATCH":  http.MethodPatch,
	"PUT":    http.MethodPut,
	"DELETE": http.MethodDelete,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	rawMethod := strings.ToUpper(salesforce.OptionalString("method", inputs))
	method, ok := allowedMethods[rawMethod]
	if !ok {
		return nil, fmt.Errorf("%q is not a method Salesforce accepts — use GET, POST, PATCH, PUT or DELETE", rawMethod)
	}

	path, err := salesforce.RequiredString("path", inputs)
	if err != nil {
		return nil, fmt.Errorf("path is required — the part of the Salesforce API to call, e.g. /limits")
	}
	path, absolute, err := normalisePath(path)
	if err != nil {
		return nil, err
	}
	path, err = appendQuery(path, inputs)
	if err != nil {
		return nil, err
	}

	var body interface{}
	if method == http.MethodPost || method == http.MethodPatch || method == http.MethodPut {
		// Pass whatever JSON the operator supplied through untouched: some
		// Salesforce endpoints (composite, collections, invocable actions) take
		// an ARRAY at the top level, so this must not assume an object.
		if body, err = salesforce.OptionalJSON("body", inputs); err != nil {
			return nil, err
		}
	}

	var resp *salesforce.APIResponse
	if absolute {
		resp, err = salesforce.ExecuteAbsolute(instanceURL, token, method, path, body)
	} else {
		resp, err = salesforce.ExecuteAPI(instanceURL, token, method, path, body)
	}
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	result := decodeBody(resp.Body)
	// Lift an id out when the endpoint returned one, so a create through this
	// action chains onward exactly like a first-class create action would.
	id := salesforce.StringifyID(result["id"])
	if id == "" {
		id = salesforce.StringifyID(result["Id"])
	}
	if id == "" && (method == http.MethodPatch || method == http.MethodPut || method == http.MethodDelete) {
		// An update or delete answers 204 No Content with no body at all, so
		// there is nothing in the response to lift an ID from — but the ID is
		// sitting in the path the operator supplied. Without this, a record
		// update made through this action cannot be chained onward, while the
		// same update made through Update Record can.
		id = recordIDFromPath(path)
	}
	summary := fmt.Sprintf("Salesforce answered %s %s with %d", method, path, resp.StatusCode)
	return salesforce.RecordResult(id, result, summary), nil
}

// recordIDPattern matches a Salesforce record ID: 15 (case-sensitive) or 18
// (case-safe) alphanumeric characters.
var recordIDPattern = regexp.MustCompile(`^[a-zA-Z0-9]{15}([a-zA-Z0-9]{3})?$`)

// recordIDFromPath recovers the record a bodiless write addressed, by reading
// the last segment of its path — /sobjects/Account/0015f00000AbCdEAAV.
//
// Returns "" for anything that is not shaped like a record ID, so a call to a
// collection endpoint (/composite/sobjects) or a named one (/limits) reports no
// ID rather than a made-up one.
func recordIDFromPath(path string) string {
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimSuffix(path, "/")
	segment := path[strings.LastIndex(path, "/")+1:]
	if unescaped, err := url.PathUnescape(segment); err == nil {
		segment = unescaped
	}
	if !recordIDPattern.MatchString(segment) {
		return ""
	}
	return segment
}

// normalisePath cleans the operator's path and decides which root it hangs off.
//
// The leading slash is not cosmetic: the path is appended to the org's origin,
// so without it a value like "@evil.example/x" would make the origin the
// userinfo of somebody else's URL and post the access token there.
func normalisePath(raw string) (path string, absolute bool, err error) {
	path = strings.TrimSpace(raw)
	if strings.Contains(path, "://") {
		return "", false, fmt.Errorf("path must be a path within your Salesforce org, not a full URL — use /limits, not https://.../limits")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// A protocol-relative path stays on the org's host once appended, but it
	// addresses nothing real, so reject it rather than issue a confusing 404.
	if strings.HasPrefix(path, "//") {
		return "", false, fmt.Errorf("path must be a single path within your Salesforce org, e.g. /limits")
	}
	// Anything already rooted at /services carries its own prefix (and its own
	// API version, for the versioned ones), so it must not be re-prefixed.
	return path, strings.HasPrefix(path, "/services/"), nil
}

// appendQuery folds the Query Parameters object into the path's query string,
// escaping each value. A path that already carries a query string keeps it —
// the two are merged rather than one silently replacing the other.
func appendQuery(path string, inputs []*core.Connection) (string, error) {
	raw, err := salesforce.OptionalJSON("query", inputs)
	if err != nil {
		return "", err
	}
	if raw == nil {
		return path, nil
	}
	params, ok := raw.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf(`query must be a JSON object of parameter names to values, e.g. {"q":"SELECT Id FROM Account"}`)
	}
	if len(params) == 0 {
		return path, nil
	}
	values := url.Values{}
	// Sorted so the same configuration always produces the same URL, which
	// makes a failing call reproducible from the summary line.
	for _, key := range salesforce.SortedKeys(params) {
		values.Set(key, fmt.Sprintf("%v", params[key]))
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + values.Encode(), nil
}

// decodeBody parses whatever Salesforce returned into the map the result output
// expects. Salesforce endpoints answer with an object, a top-level ARRAY (the
// collections and composite endpoints), or nothing at all on a 204 — all three
// are normal, so each is given a shape a downstream node can read.
func decodeBody(body []byte) map[string]interface{} {
	if len(strings.TrimSpace(string(body))) == 0 {
		return map[string]interface{}{}
	}
	var parsed interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		// Not every Salesforce endpoint returns JSON (a blob download, for
		// instance). Hand the text back rather than failing a call that worked
		// — which is why this never returns an error.
		return map[string]interface{}{"body": string(body)}
	}
	switch v := parsed.(type) {
	case map[string]interface{}:
		return v
	case []interface{}:
		return map[string]interface{}{"records": v}
	default:
		return map[string]interface{}{"body": v}
	}
}
