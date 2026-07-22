// Package oracle_logging_log_group_create creates a log group — the container that log objects
// (service and custom logs) live in within a compartment. Asynchronous: the call returns only a
// work-request id and no body; poll the work request until it succeeds before the group is usable.
package oracle_logging_log_group_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	lg "flomation.app/automate/executor/actions/oracle/logging"

	"github.com/oracle/oci-go-sdk/v65/logging"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Logging: Create Log Group"
	Description  = "Create a log group — the container that service and custom logs live in. Asynchronous: returns a work-request id and no body; poll the work request until it succeeds."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+file-lines"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the log group", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
	{Name: "freeform_tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: "{\"env\":\"prod\"} (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := lg.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}
	name, err := lg.RequiredString("display_name", inputs)
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}

	details := logging.CreateLogGroupDetails{CompartmentId: &compartment, DisplayName: &name}
	if d := lg.OptionalString("description", inputs); d != "" {
		details.Description = &d
	}
	if tags, err := lg.FreeformTags("freeform_tags", inputs); err != nil {
		return lg.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.CreateLogGroup(lg.Context(), logging.CreateLogGroupRequest{CreateLogGroupDetails: details})
	if err != nil {
		return lg.ErrorResult(auth.OCIError(err)), nil
	}
	return lg.Result(fmt.Sprintf("Creating log group %q — poll the work request until it succeeds", name), map[string]interface{}{
		"work_request_id": lg.Str(resp.OpcWorkRequestId),
	}), nil
}
