// Package oracle_blockvolume_volume_group_change_compartment moves a volume group
// to another compartment.
package oracle_blockvolume_volume_group_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: Move Volume Group to Compartment"
	Description  = "Move an Oracle Cloud volume group into a different compartment. Runs synchronously; the volume group keeps its OCID and member volumes."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+layer-group"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the volume group picker)"},
	{Name: "volume_group_ocid", Type: core.ConnectionTypeString, Label: "Volume Group OCID", Placeholder: "ocid1.volumegroup.oc1..aaaa…", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — the compartment to move the volume group to", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Volume Group OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "volume_group_ocid")
	if errResult != nil {
		return errResult, nil
	}
	dest, err := bv.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	_, err = client.ChangeVolumeGroupCompartment(bv.Context(), ocicore.ChangeVolumeGroupCompartmentRequest{
		VolumeGroupId:                       &id,
		ChangeVolumeGroupCompartmentDetails: ocicore.ChangeVolumeGroupCompartmentDetails{CompartmentId: &dest},
	})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Moved volume group %s to compartment %s", id, dest),
		"id":          id,
		"success":     true,
	}, nil
}
