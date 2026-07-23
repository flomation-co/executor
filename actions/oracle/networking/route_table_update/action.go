// Package oracle_networking_route_table_update updates the editable attributes of
// a route table — its display name and (with replace semantics) its route rules.
package oracle_networking_route_table_update

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
	Name         = "OCI Networking: Update Route Table"
	Description  = "Update editable attributes of an Oracle Cloud route table."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+pen"
	Date         = "21/07/2026"
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
	{Name: "route_table_ocid", Type: core.ConnectionTypeString, Label: "Route Table OCID", Placeholder: "ocid1.routetable.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name shown in the console (optional)"},
	{Name: "route_rules", Type: core.ConnectionTypeText, Label: "Route Rules (JSON)", Placeholder: `Replaces ALL rules when supplied, e.g. [{"networkEntityId":"ocid1.internetgateway.oc1..aaaa…","destination":"0.0.0.0/0","destinationType":"CIDR_BLOCK"}]`},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces all freeform tags when supplied (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "route_table", Type: core.ConnectionTypeObject, Label: "Route Table"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "route_table_ocid")
	if errResult != nil {
		return errResult, nil
	}
	rules, err := net.DecodeRouteRules("route_rules", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	tags, err := net.FreeformTags("tags", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	details := ocicore.UpdateRouteTableDetails{}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	// REPLACE semantics: a supplied array OVERWRITES the whole route-rule set. A
	// blank input (nil) preserves the current rules; an explicit empty array "[]"
	// clears them (an empty route table is valid) — hence != nil, not len() > 0.
	if rules != nil {
		details.RouteRules = rules
	}
	if tags != nil {
		details.FreeformTags = tags
	}
	resp, err := client.UpdateRouteTable(net.Context(), ocicore.UpdateRouteTableRequest{RtId: &id, UpdateRouteTableDetails: details})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	rt := net.SummariseRouteTable(&resp.RouteTable)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated route table %q (%s)", rt["display_name"], rt["lifecycle_state"]),
		"route_table":     rt,
		"lifecycle_state": rt["lifecycle_state"],
		"success":         true,
	}, nil
}
