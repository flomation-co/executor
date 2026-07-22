// Package oracle_functions_function_get fetches a single function by OCID and returns its full
// detail, including the lifecycle state and the invoke endpoint used to run it.
package oracle_functions_function_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	fn "flomation.app/automate/executor/actions/oracle/functions"

	"github.com/oracle/oci-go-sdk/v65/functions"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Functions: Get Function"
	Description  = "Fetch a single function by OCID, returning its full detail, lifecycle state, and invoke endpoint."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+code"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "function_ocid", Type: core.ConnectionTypeString, Label: "Function OCID", Placeholder: "ocid1.fnfunc.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "function", Type: core.ConnectionTypeObject, Label: "Function"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Function OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle state"},
	{Name: "invoke_endpoint", Type: core.ConnectionTypeString, Label: "Invoke endpoint"},
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

	resp, err := client.GetFunction(fn.Context(), functions.GetFunctionRequest{FunctionId: &functionID})
	if err != nil {
		return fn.ErrorResult(auth.OCIError(err)), nil
	}

	f := &resp.Function
	return fn.Result(fmt.Sprintf("Function %q is %s", fn.Str(f.DisplayName), string(f.LifecycleState)), map[string]interface{}{
		"function":        fn.SummariseFunction(f),
		"id":              fn.Str(f.Id),
		"lifecycle_state": string(f.LifecycleState),
		"invoke_endpoint": fn.Str(f.InvokeEndpoint),
	}), nil
}
