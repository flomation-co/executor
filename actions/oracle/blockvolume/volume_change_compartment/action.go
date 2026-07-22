// Package oracle_blockvolume_volume_change_compartment moves a block volume to
// another compartment.
package oracle_blockvolume_volume_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: Move Volume to Compartment"
	Description  = "Move an Oracle Cloud block volume into a different compartment. Runs synchronously; the volume keeps its OCID and data."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+hard-drive"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the volume picker)"},
	{Name: "volume_ocid", Type: core.ConnectionTypeString, Label: "Volume OCID", Placeholder: "ocid1.volume.oc1..aaaa…", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — the compartment to move the volume to", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Volume OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "volume_ocid")
	if errResult != nil {
		return errResult, nil
	}
	dest, err := bv.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	_, err = client.ChangeVolumeCompartment(bv.Context(), ocicore.ChangeVolumeCompartmentRequest{
		VolumeId:                       &id,
		ChangeVolumeCompartmentDetails: ocicore.ChangeVolumeCompartmentDetails{CompartmentId: &dest},
	})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Moved volume %s to compartment %s", id, dest),
		"id":          id,
		"success":     true,
	}, nil
}
