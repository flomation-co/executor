// Package oracle_compute_boot_volume_get_all lists the boot volumes in an OCI
// compartment and availability domain — the root disks instances boot from
// (including ones preserved after an instance was terminated).
package oracle_compute_boot_volume_get_all

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/oracle/compute"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Compute: List Boot Volumes"
	Description  = "List the boot volumes (instance root disks) in an Oracle Cloud compartment, optionally scoped to one availability domain. Includes volumes preserved after an instance was terminated."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+hard-drive"
	Date         = "20/07/2026"
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
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "boot_volumes", Type: core.ConnectionTypeObject, Label: "Boot Volumes"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.BlockstorageClient()
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	req := ocicore.ListBootVolumesRequest{CompartmentId: compute.StringPtr(compartment)}
	if ad := strings.TrimSpace(compute.OptionalString("availability_domain", inputs)); ad != "" {
		req.AvailabilityDomain = &ad
	}

	var vols []map[string]interface{}
	for {
		resp, err := client.ListBootVolumes(compute.Context(), req)
		if err != nil {
			return compute.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			v := &resp.Items[i]
			m := map[string]interface{}{
				"id":                  compute.Str(v.Id),
				"display_name":        compute.Str(v.DisplayName),
				"availability_domain": compute.Str(v.AvailabilityDomain),
				"lifecycle_state":     string(v.LifecycleState),
				"image_id":            compute.Str(v.ImageId),
			}
			if v.SizeInGBs != nil {
				m["size_in_gbs"] = *v.SizeInGBs
			}
			vols = append(vols, m)
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d boot volume(s)", len(vols)),
		"boot_volumes": vols,
		"count":        len(vols),
		"success":      true,
	}, nil
}
