// Package oracle_blockvolume_volume_group_get_all lists volume groups in a
// compartment, optionally narrowed to one availability domain.
package oracle_blockvolume_volume_group_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: List Volume Groups"
	Description  = "List Oracle Cloud volume groups in a compartment, optionally narrowed to one availability domain. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "Only list volume groups in this AD, e.g. Uocm:UK-LONDON-1-AD-1 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "volume_groups", Type: core.ConnectionTypeObject, Label: "Volume Groups"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := bv.GetAuth(inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	client, err := auth.BlockstorageClient()
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	req := ocicore.ListVolumeGroupsRequest{CompartmentId: &compartment}
	if ad := bv.OptionalString("availability_domain", inputs); ad != "" {
		req.AvailabilityDomain = &ad
	}
	var groups []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= bv.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListVolumeGroups(bv.Context(), req)
		if err != nil {
			return bv.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			groups = append(groups, bv.SummariseVolumeGroup(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	msg := fmt.Sprintf("Found %d volume group(s)", len(groups))
	if truncated {
		msg += " (truncated at the page cap — narrow by availability domain for the full set)"
	}
	return map[string]interface{}{
		"tool_result":   msg,
		"volume_groups": groups,
		"count":         fmt.Sprintf("%d", len(groups)),
		"truncated":     truncated,
		"success":       true,
	}, nil
}
