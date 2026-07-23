// Package oracle_functions_function_update updates attributes of an existing OCI function.
// Only the fields the operator supplies are changed — image, memory, timeout, and config are
// each applied only when a value is given, so unset fields are left untouched.
package oracle_functions_function_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	fn "flomation.app/automate/executor/actions/oracle/functions"

	"github.com/oracle/oci-go-sdk/v65/functions"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Functions: Update Function"
	Description  = "Update an existing function's image, memory, timeout, or config. Only the fields you supply are changed; everything else is left as-is."
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
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image", Placeholder: "phx.ocir.io/tenancy/repo/function:tag (optional — leave blank to keep)"},
	{Name: "memory_in_mbs", Type: core.ConnectionTypeString, Label: "Memory (MiB)", Placeholder: "Max usable memory, e.g. 256 (optional)"},
	{Name: "timeout_in_seconds", Type: core.ConnectionTypeString, Label: "Timeout (seconds)", Placeholder: "Execution timeout, e.g. 30 (optional)"},
	{Name: "config", Type: core.ConnectionTypeText, Label: "Config", Placeholder: "JSON object of env vars, e.g. {\"KEY\":\"value\"} (optional — replaces existing)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "function", Type: core.ConnectionTypeObject, Label: "Function"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Function OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle state"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	functionID, err := fn.RequiredString("function_ocid", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	auth, client, errResult := fn.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}

	details := functions.UpdateFunctionDetails{}
	if image := fn.OptionalString("image", inputs); image != "" {
		details.Image = &image
	}
	if mem, ok, err := fn.OptionalInt64("memory_in_mbs", inputs); err != nil {
		return fn.ErrorResult(err.Error()), nil
	} else if ok {
		details.MemoryInMBs = &mem
	}
	if to, ok, err := fn.OptionalInt64("timeout_in_seconds", inputs); err != nil {
		return fn.ErrorResult(err.Error()), nil
	} else if ok {
		t := int(to)
		details.TimeoutInSeconds = &t
	}
	if cfg, err := fn.ConfigMap("config", inputs); err != nil {
		return fn.ErrorResult(err.Error()), nil
	} else if cfg != nil {
		details.Config = cfg
	}

	resp, err := client.UpdateFunction(fn.Context(), functions.UpdateFunctionRequest{
		FunctionId:            &functionID,
		UpdateFunctionDetails: details,
	})
	if err != nil {
		return fn.ErrorResult(auth.OCIError(err)), nil
	}

	summary := fn.SummariseFunction(&resp.Function)
	return fn.Result(fmt.Sprintf("Updated function %q", fn.Str(resp.Function.DisplayName)), map[string]interface{}{
		"function":        summary,
		"id":              fn.Str(resp.Function.Id),
		"lifecycle_state": string(resp.Function.LifecycleState),
	}), nil
}
