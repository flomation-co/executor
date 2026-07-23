// Package oracle_identity_network_source_list lists the IAM network sources in a compartment (the tenancy).
package oracle_identity_network_source_list

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: List Network Sources"
	Description  = "List the Oracle Cloud IAM network sources in a compartment (the tenancy), optionally filtered by exact name — with their allowed public IP/CIDR and VCN source lists. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (network sources live in the root)"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name Filter", Placeholder: "Only the network source with this exact name (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "network_sources", Type: core.ConnectionTypeObject, Label: "Network Sources"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func summariseNetworkSource(n *identity.NetworkSourcesSummary) map[string]interface{} {
	var virtual []map[string]interface{}
	for i := range n.VirtualSourceList {
		v := n.VirtualSourceList[i]
		virtual = append(virtual, map[string]interface{}{
			"vcn_id":    iam.Str(v.VcnId),
			"ip_ranges": v.IpRanges,
		})
	}
	return map[string]interface{}{
		"id":                  iam.Str(n.Id),
		"name":                iam.Str(n.Name),
		"description":         iam.Str(n.Description),
		"compartment_id":      iam.Str(n.CompartmentId),
		"lifecycle_state":     string(n.LifecycleState),
		"public_source_list":  n.PublicSourceList,
		"virtual_source_list": virtual,
		"services":            n.Services,
		"freeform_tags":       n.FreeformTags,
		"time_created":        iam.FormatTime(n.TimeCreated),
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := iam.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment := auth.CompartmentOrTenancy()
	req := identity.ListNetworkSourcesRequest{CompartmentId: &compartment}
	if v := strings.TrimSpace(iam.OptionalString("name", inputs)); v != "" {
		req.Name = &v
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= iam.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListNetworkSources(iam.Context(), req)
		if err != nil {
			return iam.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, summariseNetworkSource(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return iam.Result(fmt.Sprintf("Found %d network source(s)", len(out)), map[string]interface{}{
		"network_sources": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
