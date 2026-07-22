// Package oracle_exadata_cloud_exadata_infrastructure_create provisions cloud Exadata
// infrastructure (the rack of compute + storage servers a VM cluster runs on). This is
// asynchronous — it returns the infrastructure in a PROVISIONING state plus a work-request
// id; poll Get Cloud Exadata Infrastructure until it is AVAILABLE.
package oracle_exadata_cloud_exadata_infrastructure_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Create Cloud Exadata Infrastructure"
	Description  = "Provision cloud Exadata infrastructure — the dedicated rack of compute and storage servers a VM cluster runs on. Pick the shape and the number of compute and storage servers to scale it. Asynchronous — poll Get Cloud Exadata Infrastructure until AVAILABLE."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A friendly name for the infrastructure", Required: true},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Vgqo:UK-LONDON-1-AD-1", Required: true},
	{Name: "shape", Type: core.ConnectionTypeString, Label: "Shape", Placeholder: "e.g. Exadata.X9M — the Exadata system shape", Required: true},
	{Name: "compute_count", Type: core.ConnectionTypeString, Label: "Compute Count", Placeholder: "Number of database (compute) servers", Required: true},
	{Name: "storage_count", Type: core.ConnectionTypeString, Label: "Storage Count", Placeholder: "Number of storage servers", Required: true},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "infrastructure", Type: core.ConnectionTypeObject, Label: "Infrastructure"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Infrastructure OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := exa.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	displayName, err := exa.RequiredString("display_name", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	adom, err := exa.RequiredString("availability_domain", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	shape, err := exa.RequiredString("shape", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	computeCount, err := exa.RequiredInt("compute_count", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	storageCount, err := exa.RequiredInt("storage_count", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	details := db.CreateCloudExadataInfrastructureDetails{
		CompartmentId:      &compartment,
		DisplayName:        &displayName,
		AvailabilityDomain: &adom,
		Shape:              &shape,
		ComputeCount:       &computeCount,
		StorageCount:       &storageCount,
	}
	if tags, err := exa.FreeformTags("tags", inputs); err != nil {
		return exa.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	resp, err := client.CreateCloudExadataInfrastructure(exa.Context(), db.CreateCloudExadataInfrastructureRequest{CreateCloudExadataInfrastructureDetails: details})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	infra := exa.SummariseCloudExadataInfrastructure(&resp.CloudExadataInfrastructure)
	return exa.Result(fmt.Sprintf("Provisioning Exadata infrastructure %q (%s) — poll Get Cloud Exadata Infrastructure until AVAILABLE", displayName, infra["lifecycle_state"]), map[string]interface{}{
		"infrastructure": infra, "id": infra["id"], "lifecycle_state": infra["lifecycle_state"], "work_request_id": exa.Str(resp.OpcWorkRequestId),
	}), nil
}
