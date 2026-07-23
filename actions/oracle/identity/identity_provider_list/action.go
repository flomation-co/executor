// Package oracle_identity_provider_list lists the federated identity providers configured on a tenancy.
package oracle_identity_provider_list

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
	Name         = "OCI Identity: List Identity Providers"
	Description  = "List the federated identity providers configured on an Oracle Cloud tenancy for a given federation protocol (e.g. SAML2). Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+id-badge"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (identity providers live in the root)"},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Placeholder: "Federation protocol — SAML2 (default)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "identity_providers", Type: core.ConnectionTypeObject, Label: "Identity Providers"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := iam.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}

	protoRaw := strings.TrimSpace(iam.OptionalString("protocol", inputs))
	if protoRaw == "" {
		protoRaw = "SAML2"
	}
	protocol, ok := identity.GetMappingListIdentityProvidersProtocolEnum(protoRaw)
	if !ok {
		return iam.ErrorResult(fmt.Sprintf("protocol %q is not supported (allowed: %s)", protoRaw, strings.Join(identity.GetListIdentityProvidersProtocolEnumStringValues(), ", "))), nil
	}

	compartment := auth.CompartmentOrTenancy()
	req := identity.ListIdentityProvidersRequest{Protocol: protocol, CompartmentId: &compartment}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= iam.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListIdentityProviders(iam.Context(), req)
		if err != nil {
			return iam.ErrorResult(auth.OCIError(err)), nil
		}
		for _, item := range resp.Items {
			out = append(out, map[string]interface{}{
				"id":              iam.Str(item.GetId()),
				"name":            iam.Str(item.GetName()),
				"description":     iam.Str(item.GetDescription()),
				"compartment_id":  iam.Str(item.GetCompartmentId()),
				"product_type":    iam.Str(item.GetProductType()),
				"lifecycle_state": string(item.GetLifecycleState()),
				"time_created":    iam.FormatTime(item.GetTimeCreated()),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return iam.Result(fmt.Sprintf("Found %d identity provider(s)", len(out)), map[string]interface{}{
		"identity_providers": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
