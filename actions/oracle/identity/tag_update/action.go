// Package oracle_identity_tag_update updates an IAM tag key definition within a namespace.
package oracle_identity_tag_update

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
	Name         = "OCI Identity: Update Tag Key"
	Description  = "Update an Oracle Cloud tag key definition (its description, retired flag or cost-tracking flag) within a tag namespace. Only the fields you supply are changed; the tag name is immutable."
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
	{Name: "tag_namespace_ocid", Type: core.ConnectionTypeString, Label: "Tag Namespace OCID", Placeholder: "ocid1.tagnamespace.oc1..aaaa… of the namespace holding the tag", Required: true},
	{Name: "tag_name", Type: core.ConnectionTypeString, Label: "Tag Name", Placeholder: "The tag key name to update (e.g. CostCenter)", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep the current one)"},
	{Name: "is_retired", Type: core.ConnectionTypeBoolean, Label: "Retired", Placeholder: "Retire (true) or reactivate (false) the tag key (leave unset to keep unchanged)"},
	{Name: "is_cost_tracking", Type: core.ConnectionTypeBoolean, Label: "Cost Tracking", Placeholder: "Enable (true) or disable (false) cost tracking (leave unset to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "tag", Type: core.ConnectionTypeObject, Label: "Tag Key"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Tag OCID"},
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
	tagName, err := iam.RequiredString("tag_name", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}

	// UpdateTagDetails carries only independent scalars here, so overlay just the
	// supplied inputs — a nil *field leaves that attribute unchanged.
	details := identity.UpdateTagDetails{}
	var changed []string

	if iam.BoolWasSet("description", inputs) {
		desc := strings.TrimSpace(iam.OptionalString("description", inputs))
		details.Description = &desc
		changed = append(changed, "description")
	}
	if iam.BoolWasSet("is_retired", inputs) {
		v := iam.OptionalBool("is_retired", inputs, false)
		details.IsRetired = &v
		if v {
			changed = append(changed, "retired")
		} else {
			changed = append(changed, "reactivated")
		}
	}
	if iam.BoolWasSet("is_cost_tracking", inputs) {
		v := iam.OptionalBool("is_cost_tracking", inputs, false)
		details.IsCostTracking = &v
		if v {
			changed = append(changed, "cost tracking on")
		} else {
			changed = append(changed, "cost tracking off")
		}
	}

	if len(changed) == 0 {
		return iam.ErrorResult("Set at least one of description, retired or cost tracking to update — none were provided."), nil
	}

	resp, err := client.UpdateTag(iam.Context(), identity.UpdateTagRequest{
		TagNamespaceId:   &namespaceID,
		TagName:          &tagName,
		UpdateTagDetails: details,
	})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}

	t := resp.Tag
	tag := map[string]interface{}{
		"id":                 iam.Str(t.Id),
		"name":               iam.Str(t.Name),
		"description":        iam.Str(t.Description),
		"tag_namespace_id":   iam.Str(t.TagNamespaceId),
		"tag_namespace_name": iam.Str(t.TagNamespaceName),
		"compartment_id":     iam.Str(t.CompartmentId),
		"is_retired":         t.IsRetired != nil && *t.IsRetired,
		"is_cost_tracking":   t.IsCostTracking != nil && *t.IsCostTracking,
		"lifecycle_state":    string(t.LifecycleState),
		"freeform_tags":      t.FreeformTags,
		"defined_tags":       t.DefinedTags,
		"time_created":       iam.FormatTime(t.TimeCreated),
	}
	msg := fmt.Sprintf("Updated tag key %q: %s", tag["name"], strings.Join(changed, ", "))
	return iam.Result(msg, map[string]interface{}{"tag": tag, "id": tag["id"]}), nil
}
