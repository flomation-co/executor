// Package oracle_exadata_cloud_exadata_infrastructure_change_compartment moves a cloud
// Exadata infrastructure rack into a different compartment — an asynchronous move that
// returns a work-request id to track.
package oracle_exadata_cloud_exadata_infrastructure_change_compartment

import (
	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Move Cloud Exadata Infrastructure to Compartment"
	Description  = "Move a cloud Exadata infrastructure rack into a different compartment — an asynchronous move returning a work-request id to track."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microchip"
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
	{Name: "cloud_exadata_infrastructure_ocid", Type: core.ConnectionTypeString, Label: "Cloud Exadata Infrastructure OCID", Placeholder: "ocid1.cloudexadatainfrastructure.oc1..aaaa…", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — where to move it", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Infrastructure OCID"},
	{Name: "destination_compartment_id", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := exa.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := exa.RequiredString("cloud_exadata_infrastructure_ocid", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	dest, err := exa.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	resp, err := client.ChangeCloudExadataInfrastructureCompartment(exa.Context(), db.ChangeCloudExadataInfrastructureCompartmentRequest{
		CloudExadataInfrastructureId: &id,
		ChangeCloudExadataInfrastructureCompartmentDetails: db.ChangeCloudExadataInfrastructureCompartmentDetails{CompartmentId: &dest},
	})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	return exa.Result("Moved infrastructure to destination compartment", map[string]interface{}{
		"id": id, "destination_compartment_id": dest, "work_request_id": exa.Str(resp.OpcWorkRequestId),
	}), nil
}
