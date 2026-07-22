// Package oracle_identity_tag_namespace_update updates a tag namespace's description and/or retired state.
package oracle_identity_tag_namespace_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Update Tag Namespace"
	Description  = "Update an Oracle Cloud tag namespace's description and/or retired state — only the fields you supply are changed."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+tag"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… (the caller's user, for signing)", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "the tenancy home region, e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the picker)"},
	{Name: "tag_namespace_ocid", Type: core.ConnectionTypeString, Label: "Tag Namespace OCID", Placeholder: "ocid1.tagnamespace.oc1..aaaa… of the namespace to update", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep the current one)"},
	{Name: "is_retired", Type: core.ConnectionTypeBoolean, Label: "Retired", Placeholder: "On to retire the namespace, off to reactivate (leave blank to keep)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "tag_namespace", Type: core.ConnectionTypeObject, Label: "Tag Namespace"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Tag Namespace OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := iam.ResourceClient(inputs, "tag_namespace_ocid")
	if errResult != nil {
		return errResult, nil
	}

	details := identity.UpdateTagNamespaceDetails{}
	changed := false

	if desc := iam.OptionalString("description", inputs); desc != "" {
		details.Description = &desc
		changed = true
	}
	if iam.BoolWasSet("is_retired", inputs) {
		retired := iam.OptionalBool("is_retired", inputs, false)
		details.IsRetired = &retired
		changed = true
	}

	if !changed {
		return iam.ErrorResult("nothing to update — supply a description and/or a retired state"), nil
	}

	resp, err := client.UpdateTagNamespace(iam.Context(), identity.UpdateTagNamespaceRequest{
		TagNamespaceId:            &id,
		UpdateTagNamespaceDetails: details,
	})
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
		"freeform_tags":   ns.FreeformTags,
		"defined_tags":    ns.DefinedTags,
	}
	return iam.Result(
		fmt.Sprintf("Updated tag namespace %q (%s)", tagNamespace["name"], tagNamespace["lifecycle_state"]),
		map[string]interface{}{"tag_namespace": tagNamespace, "id": tagNamespace["id"]},
	), nil
}
