// Package oracle_bastion_bastion_list lists the bastions in a compartment, optionally filtered by
// lifecycle state, exact name, or a specific bastion OCID. Walks pagination up to a safe cap.
package oracle_bastion_bastion_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	bas "flomation.app/automate/executor/actions/oracle/bastion"

	"github.com/oracle/oci-go-sdk/v65/bastion"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Bastion: List Bastions"
	Description  = "List the bastions in a compartment. Optionally filter by lifecycle state, exact name, or a specific bastion OCID. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Only bastions in this state (optional)", Options: []core.ConnectionOption{
		{Name: "Creating", Value: "CREATING"}, {Name: "Updating", Value: "UPDATING"}, {Name: "Active", Value: "ACTIVE"},
		{Name: "Deleting", Value: "DELETING"}, {Name: "Deleted", Value: "DELETED"}, {Name: "Failed", Value: "FAILED"},
	}},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name Filter", Placeholder: "Only the bastion whose name matches this exactly (optional)"},
	{Name: "bastion_id", Type: core.ConnectionTypeString, Label: "Bastion OCID Filter", Placeholder: "Restrict to a specific bastion OCID (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "bastions", Type: core.ConnectionTypeObject, Label: "Bastions"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := bas.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}
	req := bastion.ListBastionsRequest{CompartmentId: &compartment}
	if state := bas.OptionalString("lifecycle_state", inputs); state != "" {
		req.BastionLifecycleState = bastion.ListBastionsBastionLifecycleStateEnum(state)
	}
	if name := bas.OptionalString("name", inputs); name != "" {
		req.Name = &name
	}
	if id := bas.OptionalString("bastion_id", inputs); id != "" {
		req.BastionId = &id
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= bas.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListBastions(bas.Context(), req)
		if err != nil {
			return bas.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, bas.SummariseBastionSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return bas.Result(fmt.Sprintf("Found %d bastion(s)", len(out)), map[string]interface{}{
		"bastions": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
