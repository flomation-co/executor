// Package oracle_identity_network_source_get reads one IAM network source by OCID.
package oracle_identity_network_source_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Get Network Source"
	Description  = "Fetch a single Oracle Cloud IAM network source by OCID — its name, description, allowed public IP/CIDR list, VCN source list and lifecycle state."
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
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… (the caller's user, for signing)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "the tenancy home region, e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the network-source picker)"},
	{Name: "network_source_ocid", Type: core.ConnectionTypeString, Label: "Network Source OCID", Placeholder: "ocid1.networksource.oc1..aaaa… of the network source to fetch", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "network_source", Type: core.ConnectionTypeObject, Label: "Network Source"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Network Source OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := iam.ResourceClient(inputs, "network_source_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetNetworkSource(iam.Context(), identity.GetNetworkSourceRequest{NetworkSourceId: &id})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}

	ns := resp.NetworkSources
	virtual := make([]map[string]interface{}, 0, len(ns.VirtualSourceList))
	for _, v := range ns.VirtualSourceList {
		virtual = append(virtual, map[string]interface{}{
			"vcn_id":    iam.Str(v.VcnId),
			"ip_ranges": v.IpRanges,
		})
	}
	networkSource := map[string]interface{}{
		"id":                  iam.Str(ns.Id),
		"name":                iam.Str(ns.Name),
		"description":         iam.Str(ns.Description),
		"compartment_id":      iam.Str(ns.CompartmentId),
		"lifecycle_state":     string(ns.LifecycleState),
		"public_source_list":  ns.PublicSourceList,
		"virtual_source_list": virtual,
		"services":            ns.Services,
		"freeform_tags":       ns.FreeformTags,
		"defined_tags":        ns.DefinedTags,
		"time_created":        iam.FormatTime(ns.TimeCreated),
	}
	if ns.InactiveStatus != nil {
		networkSource["inactive_status"] = *ns.InactiveStatus
	}

	return iam.Result(
		fmt.Sprintf("Network source %q is %s", networkSource["name"], networkSource["lifecycle_state"]),
		map[string]interface{}{"network_source": networkSource, "id": networkSource["id"]},
	), nil
}
