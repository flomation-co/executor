// Package oracle_functions_function_invoke invokes a function synchronously with an optional
// payload and returns its response body. The operator supplies the function OCID; this action
// resolves the function's own invoke endpoint automatically, so no raw endpoint URL is needed.
package oracle_functions_function_invoke

import (
	"fmt"
	"io"
	"strings"

	core "flomation.app/automate/executor"
	fn "flomation.app/automate/executor/actions/oracle/functions"

	"github.com/oracle/oci-go-sdk/v65/functions"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Functions: Invoke Function"
	Description  = "Invoke a function by OCID with an optional payload and return its response body. The function's invoke endpoint is resolved automatically. Use dry-run to validate access without executing."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+code"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "function_ocid", Type: core.ConnectionTypeString, Label: "Function OCID", Placeholder: "ocid1.fnfunc.oc1..aaaa…", Required: true},
	{Name: "payload", Type: core.ConnectionTypeText, Label: "Payload", Placeholder: "The request body passed to the function (optional)"},
	{Name: "dry_run", Type: core.ConnectionTypeBoolean, Label: "Dry Run", Placeholder: "Validate access without running the function"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "response", Type: core.ConnectionTypeString, Label: "Function response body"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	functionID, err := fn.RequiredString("function_ocid", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	auth, client, errResult := fn.InvokeClientForFunction(inputs, functionID)
	if errResult != nil {
		return errResult, nil
	}

	req := functions.InvokeFunctionRequest{FunctionId: &functionID}
	if payload := fn.OptionalString("payload", inputs); payload != "" {
		req.InvokeFunctionBody = io.NopCloser(strings.NewReader(payload))
	}
	if p := fn.OptionalBoolPtr("dry_run", inputs); p != nil && *p {
		req.IsDryRun = p
	}

	resp, err := client.InvokeFunction(fn.Context(), req)
	if err != nil {
		return fn.ErrorResult(auth.OCIError(err)), nil
	}
	body := ""
	if resp.Content != nil {
		b, readErr := io.ReadAll(resp.Content)
		_ = resp.Content.Close()
		if readErr != nil {
			return fn.ErrorResult(fmt.Sprintf("function invoked but its response could not be read: %s", readErr.Error())), nil
		}
		body = string(b)
	}
	return fn.Result("Function invoked", map[string]interface{}{"response": body}), nil
}
