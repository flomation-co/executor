// Package oracle_logging_log_group_change_compartment moves a Logging log group from one
// compartment to another. The log group keeps its OCID; only its compartment placement (for
// access control and billing) changes.
package oracle_logging_log_group_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	lg "flomation.app/automate/executor/actions/oracle/logging"

	"github.com/oracle/oci-go-sdk/v65/logging"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Logging: Change Log Group Compartment"
	Description  = "Move a log group into a different compartment — the log group keeps its OCID, only its compartment placement changes."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+file-lines"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "log_group_ocid", Type: core.ConnectionTypeString, Label: "Log Group OCID", Placeholder: "ocid1.loggroup.oc1..aaaa… (the log group to move)", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where to move the log group)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Log Group OCID"},
	{Name: "destination_compartment_id", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := lg.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	logGroupID, err := lg.RequiredString("log_group_ocid", inputs)
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}
	destination, err := lg.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}

	_, err = client.ChangeLogGroupCompartment(lg.Context(), logging.ChangeLogGroupCompartmentRequest{
		LogGroupId: &logGroupID,
		ChangeLogGroupCompartmentDetails: logging.ChangeLogGroupCompartmentDetails{
			CompartmentId: &destination,
		},
	})
	if err != nil {
		return lg.ErrorResult(auth.OCIError(err)), nil
	}

	return lg.Result(fmt.Sprintf("Moved log group %s into compartment %s", logGroupID, destination), map[string]interface{}{
		"id":                         logGroupID,
		"destination_compartment_id": destination,
	}), nil
}
