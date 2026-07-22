// Package oracle_identity_compartment_get reads one IAM compartment by OCID.
package oracle_identity_compartment_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Get Compartment"
	Description  = "Fetch a single Oracle Cloud IAM compartment by OCID — its name, description, parent, accessibility and lifecycle state."
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
	{Name: "target_compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID (to read)", Placeholder: "ocid1.compartment.oc1..aaaa… of the compartment to fetch", Required: true},
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
	resp, err := client.GetCompartment(iam.Context(), identity.GetCompartmentRequest{CompartmentId: &id})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	compartment := iam.SummariseCompartment(&resp.Compartment)
	return iam.Result(fmt.Sprintf("Compartment %q is %s", compartment["name"], compartment["lifecycle_state"]), map[string]interface{}{"compartment": compartment, "id": compartment["id"]}), nil
}
