// Package oracle_cloudguard_managed_list_delete deletes a Cloud Guard managed list by its OCID,
// removing the parameter list from any detector or responder rules that referenced it.
package oracle_cloudguard_managed_list_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	cg "flomation.app/automate/executor/actions/oracle/cloudguard"

	"github.com/oracle/oci-go-sdk/v65/cloudguard"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Cloud Guard: Delete Managed List"
	Description  = "Delete a Cloud Guard managed list by its OCID — it is removed from any rules that referenced it."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
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
	{Name: "managed_list_ocid", Type: core.ConnectionTypeString, Label: "Managed List OCID", Placeholder: "ocid1.cloudguardmanagedlist.oc1..aaaa… of the list to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Managed List OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := cg.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	listID, err := cg.RequiredString("managed_list_ocid", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}

	_, err = client.DeleteManagedList(cg.Context(), cloudguard.DeleteManagedListRequest{ManagedListId: &listID})
	if err != nil {
		return cg.ErrorResult(auth.OCIError(err)), nil
	}
	return cg.Result(fmt.Sprintf("Deleted managed list %s", listID), map[string]interface{}{
		"id": listID,
	}), nil
}
