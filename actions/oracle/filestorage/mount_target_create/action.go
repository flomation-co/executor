// Package oracle_filestorage_mount_target_create creates a mount target — the NFS
// endpoint (a private IP in a subnet) that clients mount file systems through. Creating
// one also creates its export set. Synchronous-ish; poll Get Mount Target until ACTIVE.
package oracle_filestorage_mount_target_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Create Mount Target"
	Description  = "Create an Oracle Cloud mount target — the NFS endpoint (a private IP in a subnet) that clients mount file systems through. Also creates its export set. Wire file systems to it with Create Export. Poll Get Mount Target until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+network-wired"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (must match the subnet's AD)", Required: true},
	{Name: "subnet_ocid", Type: core.ConnectionTypeString, Label: "Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… the mount target's private IP lives in", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A friendly name for the mount target", Required: true},
	{Name: "ip_address", Type: core.ConnectionTypeString, Label: "IP Address", Placeholder: "Optional fixed private IP in the subnet (defaults to auto-assign)"},
	{Name: "nsg_ocids", Type: core.ConnectionTypeString, Label: "NSG OCIDs (comma-separated)", Placeholder: "ocid1.networksecuritygroup…,… (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "mount_target", Type: core.ConnectionTypeObject, Label: "Mount Target"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Mount Target OCID"},
	{Name: "export_set_id", Type: core.ConnectionTypeString, Label: "Export Set OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := fss.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	ad, err := fss.RequiredAvailabilityDomain(inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	subnet, err := fss.RequiredString("subnet_ocid", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	displayName, err := fss.RequiredString("display_name", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	details := filestorage.CreateMountTargetDetails{CompartmentId: &compartment, AvailabilityDomain: &ad, SubnetId: &subnet, DisplayName: &displayName}
	if v := strings.TrimSpace(fss.OptionalString("ip_address", inputs)); v != "" {
		details.IpAddress = &v
	}
	if nsgs := fss.InputStrings("nsg_ocids", inputs); len(nsgs) > 0 {
		details.NsgIds = nsgs
	}
	if tags, err := fss.FreeformTags("tags", inputs); err != nil {
		return fss.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	resp, err := client.CreateMountTarget(fss.Context(), filestorage.CreateMountTargetRequest{CreateMountTargetDetails: details})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}
	mt := fss.SummariseMountTarget(&resp.MountTarget)
	return fss.Result(fmt.Sprintf("Creating mount target %q (%s) — poll Get Mount Target until ACTIVE", displayName, mt["lifecycle_state"]), map[string]interface{}{
		"mount_target": mt, "id": mt["id"], "export_set_id": mt["export_set_id"], "lifecycle_state": mt["lifecycle_state"],
	}), nil
}
