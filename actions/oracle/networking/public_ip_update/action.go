// Package oracle_networking_public_ip_update edits the mutable attributes of a
// public IP — its display name, the private IP it is assigned to, and freeform
// tags.
package oracle_networking_public_ip_update

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
	Name         = "OCI Networking: Update Public IP"
	Description  = "Update editable attributes of an Oracle Cloud public IP."
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
	{Name: "public_ip_ocid", Type: core.ConnectionTypeString, Label: "Public IP OCID", Placeholder: "ocid1.publicip.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name shown in the console (optional)"},
	{Name: "private_ip_ocid", Type: core.ConnectionTypeString, Label: "Private IP OCID", Placeholder: "ocid1.privateip.oc1..aaaa… — assign/reassign the public IP to this private IP (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces existing freeform tags (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "public_ip", Type: core.ConnectionTypeObject, Label: "Public IP"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "public_ip_ocid")
	if errResult != nil {
		return errResult, nil
	}
	tags, err := net.FreeformTags("tags", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	details := ocicore.UpdatePublicIpDetails{}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(net.OptionalString("private_ip_ocid", inputs)); v != "" {
		details.PrivateIpId = &v
	}
	if tags != nil {
		details.FreeformTags = tags
	}
	resp, err := client.UpdatePublicIp(net.Context(), ocicore.UpdatePublicIpRequest{PublicIpId: &id, UpdatePublicIpDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	publicIP := net.SummarisePublicIp(&resp.PublicIp)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated public IP %q (%s)", publicIP["display_name"], publicIP["lifecycle_state"]),
		"public_ip":       publicIP,
		"lifecycle_state": publicIP["lifecycle_state"],
		"success":         true,
	}, nil
}
