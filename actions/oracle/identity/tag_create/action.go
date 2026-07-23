// Package oracle_identity_tag_create creates a defined tag key inside a tag namespace.
package oracle_identity_tag_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Create Tag Key"
	Description  = "Create a defined tag key inside an Oracle Cloud tag namespace. The name is unique within the namespace and cannot be changed later; enable cost tracking to have it appear on cost reports."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the picker)"},
	{Name: "tag_namespace_ocid", Type: core.ConnectionTypeString, Label: "Tag Namespace OCID", Placeholder: "ocid1.tagnamespace.oc1..aaaa… the tag key belongs to", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Tag Key Name", Placeholder: "Unique in the namespace, e.g. CostCenter (cannot be changed later)", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this tag key is for", Required: true},
	{Name: "is_cost_tracking", Type: core.ConnectionTypeBoolean, Label: "Cost Tracking", Placeholder: "Enable this tag for cost tracking (default off)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "tag", Type: core.ConnectionTypeObject, Label: "Tag Key"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Tag Key OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := iam.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	namespaceID, err := iam.RequiredString("tag_namespace_ocid", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	name, err := iam.RequiredString("name", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	description, err := iam.RequiredString("description", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	costTracking := iam.OptionalBool("is_cost_tracking", inputs, false)
	details := identity.CreateTagDetails{
		Name:           &name,
		Description:    &description,
		IsCostTracking: &costTracking,
	}
	resp, err := client.CreateTag(iam.Context(), identity.CreateTagRequest{
		TagNamespaceId:   &namespaceID,
		CreateTagDetails: details,
	})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	t := resp.Tag
	tag := map[string]interface{}{
		"id":                 iam.Str(t.Id),
		"name":               iam.Str(t.Name),
		"description":        iam.Str(t.Description),
		"compartment_id":     iam.Str(t.CompartmentId),
		"tag_namespace_id":   iam.Str(t.TagNamespaceId),
		"tag_namespace_name": iam.Str(t.TagNamespaceName),
		"is_retired":         t.IsRetired != nil && *t.IsRetired,
		"is_cost_tracking":   t.IsCostTracking != nil && *t.IsCostTracking,
		"lifecycle_state":    string(t.LifecycleState),
		"freeform_tags":      t.FreeformTags,
		"defined_tags":       t.DefinedTags,
		"time_created":       iam.FormatTime(t.TimeCreated),
	}
	return iam.Result(fmt.Sprintf("Created tag key %q in namespace %s", name, tag["tag_namespace_name"]), map[string]interface{}{"tag": tag, "id": tag["id"]}), nil
}
