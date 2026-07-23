// Package oracle_identity_network_source_create creates an IAM network source (allowed source IPs/CIDRs).
package oracle_identity_network_source_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Create Network Source"
	Description  = "Create an Oracle Cloud IAM network source — a named list of allowed public IPs/CIDR ranges you can reference in policy statements to restrict access by source IP."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (network sources live in the root compartment)"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Unique network-source name (cannot be changed later)", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this network source is for", Required: true},
	{Name: "public_source_list", Type: core.ConnectionTypeString, Label: "Public Source List", Placeholder: "Comma-separated public IPs / CIDR ranges, e.g. 129.213.39.0/24, 203.0.113.5"},
	{Name: "services", Type: core.ConnectionTypeString, Label: "Services", Placeholder: "Comma-separated services, e.g. all (reserved by Oracle — usually left blank)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "network_source", Type: core.ConnectionTypeObject, Label: "Network Source"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Network Source OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := iam.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	name, err := iam.RequiredString("name", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	description, err := iam.RequiredString("description", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}

	compartment := auth.CompartmentOrTenancy()
	details := identity.CreateNetworkSourceDetails{
		CompartmentId:    &compartment,
		Name:             &name,
		Description:      &description,
		PublicSourceList: iam.InputStrings("public_source_list", inputs),
		Services:         iam.InputStrings("services", inputs),
	}

	resp, err := client.CreateNetworkSource(iam.Context(), identity.CreateNetworkSourceRequest{CreateNetworkSourceDetails: details})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}

	ns := resp.NetworkSources
	source := map[string]interface{}{
		"id":                  iam.Str(ns.Id),
		"name":                iam.Str(ns.Name),
		"description":         iam.Str(ns.Description),
		"compartment_id":      iam.Str(ns.CompartmentId),
		"lifecycle_state":     string(ns.LifecycleState),
		"public_source_list":  ns.PublicSourceList,
		"virtual_source_list": ns.VirtualSourceList,
		"services":            ns.Services,
		"time_created":        iam.FormatTime(ns.TimeCreated),
	}

	return iam.Result(
		fmt.Sprintf("Created network source %q (%s)", source["name"], source["lifecycle_state"]),
		map[string]interface{}{"network_source": source, "id": source["id"]},
	), nil
}
