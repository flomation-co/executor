// Package oracle_blockvolume_boot_volume_attachment_get_all lists boot-volume
// attachments in a compartment and availability domain, optionally narrowed to one
// instance or one boot volume.
package oracle_blockvolume_boot_volume_attachment_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: List Boot Volume Attachments"
	Description  = "List Oracle Cloud boot-volume attachments in a compartment and availability domain, optionally narrowed to one instance or one boot volume. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+plug"
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
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1", Required: true},
	{Name: "instance_ocid", Type: core.ConnectionTypeString, Label: "Instance OCID", Placeholder: "Only attachments for this instance, ocid1.instance.oc1..aaaa… (optional)"},
	{Name: "boot_volume_ocid", Type: core.ConnectionTypeString, Label: "Boot Volume OCID", Placeholder: "Only attachments for this boot volume, ocid1.bootvolume.oc1..aaaa… (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "attachments", Type: core.ConnectionTypeObject, Label: "Boot Volume Attachments"},
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
	ad, err := bv.RequiredString("availability_domain", inputs)
	if err != nil {
		return bv.ErrorResult(err.Error()), nil
	}
	client, err := auth.ComputeClient()
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	req := ocicore.ListBootVolumeAttachmentsRequest{
		AvailabilityDomain: &ad,
		CompartmentId:      &compartment,
	}
	if instance := bv.OptionalString("instance_ocid", inputs); instance != "" {
		req.InstanceId = &instance
	}
	if bootVolume := bv.OptionalString("boot_volume_ocid", inputs); bootVolume != "" {
		req.BootVolumeId = &bootVolume
	}
	var attachments []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= bv.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListBootVolumeAttachments(bv.Context(), req)
		if err != nil {
			return bv.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			attachments = append(attachments, bv.SummariseBootVolumeAttachment(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	msg := fmt.Sprintf("Found %d boot volume attachment(s)", len(attachments))
	if truncated {
		msg += " (truncated at the page cap — narrow by instance or boot volume for the full set)"
	}
	return map[string]interface{}{
		"tool_result": msg,
		"attachments": attachments,
		"count":       fmt.Sprintf("%d", len(attachments)),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
