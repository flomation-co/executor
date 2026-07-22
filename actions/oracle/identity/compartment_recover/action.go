// Package oracle_identity_compartment_recover un-deletes a DELETED IAM compartment.
package oracle_identity_compartment_recover

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Recover Compartment"
	Description  = "Recover (un-delete) a previously deleted Oracle Cloud IAM compartment by OCID, returning it to the active state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+folder-tree"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… (the caller's user, for signing)", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "the tenancy home region, e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the compartment picker)"},
	{Name: "target_compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID (to recover)", Placeholder: "ocid1.compartment.oc1..aaaa… of the deleted compartment", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "compartment", Type: core.ConnectionTypeObject, Label: "Compartment"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Compartment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := iam.ResourceClient(inputs, "target_compartment_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.RecoverCompartment(iam.Context(), identity.RecoverCompartmentRequest{CompartmentId: &id})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	compartment := iam.SummariseCompartment(&resp.Compartment)
	return iam.Result(fmt.Sprintf("Recovered compartment %q — now %s", compartment["name"], compartment["lifecycle_state"]), map[string]interface{}{"compartment": compartment, "id": compartment["id"]}), nil
}
