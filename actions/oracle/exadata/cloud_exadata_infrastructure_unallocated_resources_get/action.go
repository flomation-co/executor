// Package oracle_exadata_cloud_exadata_infrastructure_unallocated_resources_get reports
// the spare capacity of one cloud Exadata infrastructure — the unallocated local storage,
// OCPUs, memory and Exadata storage still available to place VM clusters on.
package oracle_exadata_cloud_exadata_infrastructure_unallocated_resources_get

import (
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Get Unallocated Resources"
	Description  = "Report the spare capacity of a cloud Exadata infrastructure — unallocated local storage, OCPUs, memory and Exadata storage still available."
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Infrastructure OCID"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Infrastructure Display Name"},
	{Name: "local_storage_gb", Type: core.ConnectionTypeInteger, Label: "Unallocated Local Storage (GB)"},
	{Name: "ocpus", Type: core.ConnectionTypeInteger, Label: "Unallocated OCPUs"},
	{Name: "memory_gb", Type: core.ConnectionTypeInteger, Label: "Unallocated Memory (GB)"},
	{Name: "exadata_storage_tb", Type: core.ConnectionTypeString, Label: "Unallocated Exadata Storage (TB)"},
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
	resp, err := client.GetCloudExadataInfrastructureUnallocatedResources(exa.Context(), db.GetCloudExadataInfrastructureUnallocatedResourcesRequest{CloudExadataInfrastructureId: &id})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	r := resp.CloudExadataInfrastructureUnallocatedResources
	// ExadataStorageInTBs is a *float64 (capacity can be fractional, e.g. 49.5) — render it as a
	// string so the value isn't truncated to a whole number by a numeric output type.
	exadataStorage := ""
	if r.ExadataStorageInTBs != nil {
		exadataStorage = strconv.FormatFloat(*r.ExadataStorageInTBs, 'g', -1, 64)
	}
	return exa.Result(fmt.Sprintf("Unallocated resources for %q", exa.Str(r.CloudExadataInfrastructureDisplayName)), map[string]interface{}{
		"id":                 exa.Str(r.CloudExadataInfrastructureId),
		"display_name":       exa.Str(r.CloudExadataInfrastructureDisplayName),
		"local_storage_gb":   exa.IntOrNil(r.LocalStorageInGbs),
		"ocpus":              exa.IntOrNil(r.Ocpus),
		"memory_gb":          exa.IntOrNil(r.MemoryInGBs),
		"exadata_storage_tb": exadataStorage,
	}), nil
}
