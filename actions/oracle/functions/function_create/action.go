// Package oracle_functions_function_create creates a new function inside a Functions application.
// A function is defined by the Docker image it runs and the memory it is granted; it inherits its
// compartment and network settings from the parent application. Optionally set a timeout and
// function-level config that overrides the application's shared config.
package oracle_functions_function_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	fn "flomation.app/automate/executor/actions/oracle/functions"

	"github.com/oracle/oci-go-sdk/v65/functions"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Functions: Create Function"
	Description  = "Create a function in a Functions application from a Docker image, setting its memory and optional timeout and config."
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
	{Name: "application_id", Type: core.ConnectionTypeString, Label: "Application OCID", Placeholder: "ocid1.fnapp.oc1..aaaa… (the application to create the function in)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name unique within the application", Required: true},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image", Placeholder: "e.g. phx.ocir.io/ten/functions/function:0.0.1", Required: true},
	{Name: "memory_in_mbs", Type: core.ConnectionTypeString, Label: "Memory (MiB)", Placeholder: "Maximum usable memory, e.g. 128", Required: true},
	{Name: "timeout_in_seconds", Type: core.ConnectionTypeString, Label: "Timeout (seconds)", Placeholder: "Execution timeout in seconds (optional)"},
	{Name: "config", Type: core.ConnectionTypeText, Label: "Config", Placeholder: "Function config as a JSON object, e.g. {\"MY_KEY\":\"value\"} (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "function", Type: core.ConnectionTypeObject, Label: "Function"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Function OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "invoke_endpoint", Type: core.ConnectionTypeString, Label: "Invoke Endpoint"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := fn.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	applicationID, err := fn.RequiredString("application_id", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	displayName, err := fn.RequiredString("display_name", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	image, err := fn.RequiredString("image", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	memory, memoryOK, err := fn.OptionalInt64("memory_in_mbs", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	if !memoryOK {
		return fn.ErrorResult("memory in mbs is required"), nil
	}
	config, err := fn.ConfigMap("config", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}

	details := functions.CreateFunctionDetails{
		ApplicationId: &applicationID,
		DisplayName:   &displayName,
		Image:         &image,
		MemoryInMBs:   &memory,
	}
	if timeout, timeoutOK, terr := fn.OptionalInt64("timeout_in_seconds", inputs); terr != nil {
		return fn.ErrorResult(terr.Error()), nil
	} else if timeoutOK {
		t := int(timeout)
		details.TimeoutInSeconds = &t
	}
	if len(config) > 0 {
		details.Config = config
	}

	resp, err := client.CreateFunction(fn.Context(), functions.CreateFunctionRequest{CreateFunctionDetails: details})
	if err != nil {
		return fn.ErrorResult(auth.OCIError(err)), nil
	}
	function := fn.SummariseFunction(&resp.Function)
	return fn.Result(fmt.Sprintf("Created function %q", displayName), map[string]interface{}{
		"function":        function,
		"id":              fn.Str(resp.Id),
		"lifecycle_state": string(resp.LifecycleState),
		"invoke_endpoint": fn.Str(resp.InvokeEndpoint),
	}), nil
}
