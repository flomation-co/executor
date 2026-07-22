// Package oracle_blockvolume_boot_volume_backup_get_all lists boot-volume backups
// in a compartment, optionally narrowed to one source boot volume.
package oracle_blockvolume_boot_volume_backup_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: List Boot Volume Backups"
	Description  = "List Oracle Cloud boot-volume backups in a compartment, optionally narrowed to one boot volume. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+floppy-disk"
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
	{Name: "boot_volume_ocid", Type: core.ConnectionTypeString, Label: "Boot Volume OCID", Placeholder: "Only list backups of this boot volume, e.g. ocid1.bootvolume.oc1..aaaa… (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "boot_volume_backups", Type: core.ConnectionTypeObject, Label: "Boot Volume Backups"},
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
	req := ocicore.ListBootVolumeBackupsRequest{CompartmentId: &compartment}
	if bvID := bv.OptionalString("boot_volume_ocid", inputs); bvID != "" {
		req.BootVolumeId = &bvID
	}
	var backups []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= bv.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListBootVolumeBackups(bv.Context(), req)
		if err != nil {
			return bv.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			backups = append(backups, bv.SummariseBootVolumeBackup(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	msg := fmt.Sprintf("Found %d boot volume backup(s)", len(backups))
	if truncated {
		msg += " (truncated at the page cap — narrow by boot volume for the full set)"
	}
	return map[string]interface{}{
		"tool_result":         msg,
		"boot_volume_backups": backups,
		"count":               fmt.Sprintf("%d", len(backups)),
		"truncated":           truncated,
		"success":             true,
	}, nil
}
