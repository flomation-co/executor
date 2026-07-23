// Package oracle_identity_tag_namespace_create creates a tag namespace in the tenancy.
package oracle_identity_tag_namespace_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Create Tag Namespace"
	Description  = "Create an Oracle Cloud tag namespace — the container that holds defined tag keys. The name is unique in the tenancy and cannot be changed later."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+tag"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (tag namespaces live in the root)"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Namespace Name", Placeholder: "Unique in the tenancy, e.g. Operations (cannot be changed later)", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this tag namespace is for", Required: true},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"team":"ops"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "tag_namespace", Type: core.ConnectionTypeObject, Label: "Tag Namespace"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Tag Namespace OCID"},
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
	tags, err := iam.FreeformTags("tags", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	compartment := auth.CompartmentOrTenancy()
	details := identity.CreateTagNamespaceDetails{
		CompartmentId: &compartment,
		Name:          &name,
		Description:   &description,
		FreeformTags:  tags,
	}
	resp, err := client.CreateTagNamespace(iam.Context(), identity.CreateTagNamespaceRequest{CreateTagNamespaceDetails: details})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	ns := resp.TagNamespace
	tagNamespace := map[string]interface{}{
		"id":              iam.Str(ns.Id),
		"name":            iam.Str(ns.Name),
		"description":     iam.Str(ns.Description),
		"compartment_id":  iam.Str(ns.CompartmentId),
		"is_retired":      ns.IsRetired != nil && *ns.IsRetired,
		"lifecycle_state": string(ns.LifecycleState),
		"time_created":    iam.FormatTime(ns.TimeCreated),
	}
	return iam.Result(fmt.Sprintf("Created tag namespace %q", name), map[string]interface{}{"tag_namespace": tagNamespace, "id": tagNamespace["id"]}), nil
}
