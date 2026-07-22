// Package oracle_functions_triggers_list lists the pre-built-function (PBF) trigger types — the
// small catalog of service sources that can activate a PBF. It walks pagination up to a safe cap.
package oracle_functions_triggers_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	fn "flomation.app/automate/executor/actions/oracle/functions"

	"github.com/oracle/oci-go-sdk/v65/functions"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Functions: List Triggers"
	Description  = "List the pre-built-function (PBF) trigger types — the catalog of service sources that can activate a PBF. Optionally filter by name."
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name Filter", Placeholder: "Only trigger types matching this service source (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "triggers", Type: core.ConnectionTypeObject, Label: "Triggers"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := fn.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}

	req := functions.ListTriggersRequest{}
	if name := fn.OptionalString("name", inputs); name != "" {
		req.Name = &name
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= fn.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListTriggers(fn.Context(), req)
		if err != nil {
			return fn.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, map[string]interface{}{"name": fn.Str(resp.Items[i].Name)})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return fn.Result(fmt.Sprintf("Found %d trigger type(s)", len(out)), map[string]interface{}{
		"triggers": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
