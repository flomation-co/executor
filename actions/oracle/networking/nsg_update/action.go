// Package oracle_networking_nsg_update edits the mutable attributes of a
// Network Security Group (NSG) — its display name and freeform tags. The
// contained security rules are managed through the separate NSG rule actions.
package oracle_networking_nsg_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Update NSG"
	Description  = "Update editable attributes of an Oracle Cloud NSG."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+pen"
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
	{Name: "nsg_ocid", Type: core.ConnectionTypeString, Label: "NSG OCID", Placeholder: "ocid1.networksecuritygroup.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name shown in the console (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces existing freeform tags (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "nsg", Type: core.ConnectionTypeObject, Label: "NSG"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "nsg_ocid")
	if errResult != nil {
		return errResult, nil
	}
	tags, err := net.FreeformTags("tags", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	details := ocicore.UpdateNetworkSecurityGroupDetails{}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if tags != nil {
		details.FreeformTags = tags
	}
	resp, err := client.UpdateNetworkSecurityGroup(net.Context(), ocicore.UpdateNetworkSecurityGroupRequest{
		NetworkSecurityGroupId:            &id,
		UpdateNetworkSecurityGroupDetails: details,
	})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	nsg := net.SummariseNsg(&resp.NetworkSecurityGroup)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated NSG %q (%s)", nsg["display_name"], nsg["lifecycle_state"]),
		"nsg":             nsg,
		"lifecycle_state": nsg["lifecycle_state"],
		"success":         true,
	}, nil
}
