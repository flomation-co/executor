// Package crm_salesforce_flow_invoke runs an autolaunched Salesforce Flow.
//
// This is the one Salesforce surface that is neither /sobjects nor /query: a
// Flow is automation the org's own admin has already built, named and tested,
// so handing work to it is safer than reproducing the same logic as a chain of
// record writes. "Deal closed → run the admin's Onboarding flow" becomes one
// node instead of six, and it keeps working when the admin changes the flow.
package crm_salesforce_flow_invoke

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Run Flow"
	Description  = "Run one of your Salesforce flows and pass it the values it asks for. Use this to trigger automation your Salesforce administrator has already built."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+bolt"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "flow_name", Type: core.ConnectionTypeString, Label: "Flow", Placeholder: "Send_Welcome_Email — the flow's API Name from Setup, Flows", Required: true},
	{Name: "variable_name", Type: core.ConnectionTypeString, Label: "Value to Send", Placeholder: "recordId — the name of the value the flow asks for"},
	{Name: "variable_value", Type: core.ConnectionTypeString, Label: "Value", Placeholder: "0035f00000AbCdEAAV"},
	{Name: "variables", Type: core.ConnectionTypeObject, Label: "All Values (JSON)", Placeholder: `{"recordId":"0035f00000AbCdEAAV","SendEmail":true,"Discount":15}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Flow Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// actionError is one entry of an invocable action's error array. It is NOT the
// same shape as the REST error envelope common.go decodes: invocable actions
// report the code as "statusCode", while the sObject endpoints call it
// "errorCode". Both keys are read so neither surface loses its message.
type actionError struct {
	StatusCode string   `json:"statusCode"`
	ErrorCode  string   `json:"errorCode"`
	Message    string   `json:"message"`
	Fields     []string `json:"fields"`
}

// actionResult is one entry of the Invocable Actions response array.
type actionResult struct {
	ActionName   string                 `json:"actionName"`
	IsSuccess    bool                   `json:"isSuccess"`
	Errors       []actionError          `json:"errors"`
	OutputValues map[string]interface{} `json:"outputValues"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	flowName, err := salesforce.RequiredString("flow_name", inputs)
	if err != nil {
		return nil, err
	}
	// The flow's API name goes straight into the request path, so it is
	// whitelist-validated exactly like an sObject name — same character set,
	// same reason. Operators routinely paste the flow's LABEL ("Send Welcome
	// Email") instead of its API name, so say which one is wanted.
	apiName, err := salesforce.ValidateSOQLObjectName(flowName)
	if err != nil {
		return nil, fmt.Errorf("%q is not a Salesforce flow API name — copy the API Name column from Setup, Flows (letters, numbers and underscores only, e.g. Send_Welcome_Email)", flowName)
	}

	variables, err := buildVariables(inputs)
	if err != nil {
		return nil, err
	}

	// The Invocable Actions API always takes an ARRAY of input sets, even for a
	// single run — {"inputs":[{...}]}. Salesforce would accept several sets in
	// one call, but a Flomation node runs one flow per node run; batching would
	// hide which set failed, and the Loop node covers the repeat case.
	body := map[string]interface{}{"inputs": []interface{}{variables}}

	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodPost, "/actions/custom/flow/"+url.PathEscape(apiName), body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Decode the action-result array BEFORE the status check: a flow that runs
	// and fails answers 400 with that same array, and its per-action errors are
	// far more useful than the raw body CheckResponse would otherwise quote.
	results, decodeErr := decodeActionResults(resp.Body)
	if err := salesforce.CheckResponse(resp); err != nil {
		if msg := failureMessage(results); msg != "" {
			return salesforce.ErrorResult(msg), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}
	if decodeErr != nil {
		return salesforce.ErrorResult(decodeErr.Error()), nil
	}
	if len(results) == 0 {
		return salesforce.ErrorResult(fmt.Sprintf("Salesforce ran %s but returned no result — check the flow is active and is an autolaunched flow (screen flows cannot be run from an automation)", apiName)), nil
	}

	// A flow can fail with a 200: Salesforce reports a caught fault as
	// isSuccess=false inside a successful HTTP response, so this check is not
	// belt-and-braces — without it a failed flow would land on the success port.
	first := results[0]
	if !first.IsSuccess {
		if msg := failureMessage(results); msg != "" {
			return salesforce.ErrorResult(msg), nil
		}
		return salesforce.ErrorResult(fmt.Sprintf("the %s flow reported a failure but gave no reason — check its fault path in Setup, Flows", apiName)), nil
	}

	raw := rawResult(resp.Body)
	summary := fmt.Sprintf("Ran the %s flow", apiName)
	if n := len(first.OutputValues); n > 0 {
		summary = fmt.Sprintf("Ran the %s flow — it returned %d value(s): %s", apiName, n, strings.Join(salesforce.SortedKeys(first.OutputValues), ", "))
	}
	// A flow that creates a record conventionally hands the new ID back in a
	// recordId output variable. Surfacing it as the action's ID output is what
	// lets the next node chain off "run the flow, then update what it made".
	return salesforce.RecordResult(outputRecordID(first.OutputValues), raw, summary), nil
}

// buildVariables assembles the flow's input variables.
//
// Two ways in, because flow inputs are typed and the editor's plain text fields
// are not: the name/value pair covers the overwhelmingly common "pass one
// record ID" case, while the JSON object is the only way to send a number,
// a checkbox or a date the flow will accept without coercion. The JSON object
// is applied first so a name/value pair wins on a clash — the specific input
// beating the bulk one is the order operators expect.
func buildVariables(inputs []*core.Connection) (map[string]interface{}, error) {
	variables := map[string]interface{}{}
	if err := salesforce.MergeJSONObject(variables, inputs, "variables"); err != nil {
		return nil, err
	}

	name := salesforce.OptionalString("variable_name", inputs)
	value := salesforce.OptionalString("variable_value", inputs)
	if name == "" && value != "" {
		return nil, fmt.Errorf("variable_value was set without variable_name — Salesforce needs the name of the flow value to fill in, e.g. recordId")
	}
	if name != "" {
		variables[name] = value
	}
	return variables, nil
}

// decodeActionResults parses the Invocable Actions response array.
func decodeActionResults(body []byte) ([]actionResult, error) {
	var results []actionResult
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("failed to parse the Salesforce flow response: %w", err)
	}
	return results, nil
}

// rawResult returns the first element of the response array as a plain map so
// the action's result output carries exactly what Salesforce sent. The array
// wrapper is dropped: it only ever holds one entry because only one input set
// is ever sent, and unwrapping it saves every downstream node an index.
func rawResult(body []byte) map[string]interface{} {
	var raw []map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || len(raw) == 0 {
		return map[string]interface{}{}
	}
	return raw[0]
}

// failureMessage renders the errors from every returned action result into one
// operator-readable line. Returns "" when there is nothing to report.
func failureMessage(results []actionResult) string {
	parts := make([]string, 0, len(results))
	for _, r := range results {
		for _, e := range r.Errors {
			msg := strings.TrimSpace(e.Message)
			code := e.StatusCode
			if code == "" {
				code = e.ErrorCode
			}
			if msg == "" {
				msg = code
			} else if code != "" {
				msg = msg + " (" + code + ")"
			}
			if len(e.Fields) > 0 {
				msg += " — field(s): " + strings.Join(e.Fields, ", ")
			}
			if msg != "" {
				parts = append(parts, msg)
			}
		}
	}
	return strings.Join(parts, "; ")
}

// outputRecordID picks a record ID out of the flow's output variables.
//
// Only the conventional names are considered, and only when the value actually
// looks like a Salesforce ID — guessing wrong here would put a random string in
// the ID output and send the next node looking for a record that never existed.
func outputRecordID(outputs map[string]interface{}) string {
	for _, key := range []string{"recordId", "RecordId", "recordID", "Id", "id"} {
		v, ok := outputs[key]
		if !ok {
			continue
		}
		s := salesforce.StringifyID(v)
		if salesforce.ValidateRecordID(s) == nil {
			return s
		}
	}
	return ""
}
