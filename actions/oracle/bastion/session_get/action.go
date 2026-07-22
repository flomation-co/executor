// Package oracle_bastion_session_get fetches a single Bastion session by OCID, returning its
// target-resource details, TTL, key type and lifecycle state.
package oracle_bastion_session_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	bas "flomation.app/automate/executor/actions/oracle/bastion"

	"github.com/oracle/oci-go-sdk/v65/bastion"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Bastion: Get Session"
	Description  = "Fetch a single Bastion session by its OCID — its target-resource details, TTL, key type and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+terminal"
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
	{Name: "session_ocid", Type: core.ConnectionTypeString, Label: "Session OCID", Placeholder: "ocid1.bastionsession.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "session", Type: core.ConnectionTypeObject, Label: "Session"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Session OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := bas.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	sessionID, err := bas.RequiredString("session_ocid", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetSession(bas.Context(), bastion.GetSessionRequest{SessionId: &sessionID})
	if err != nil {
		return bas.ErrorResult(auth.OCIError(err)), nil
	}
	session := bas.SummariseSession(&resp.Session)
	return bas.Result(fmt.Sprintf("Session %q (%s)", session["display_name"], session["lifecycle_state"]), map[string]interface{}{
		"session": session, "id": session["id"], "lifecycle_state": session["lifecycle_state"],
	}), nil
}
