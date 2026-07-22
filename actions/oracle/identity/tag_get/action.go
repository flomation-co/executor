// Package oracle_identity_tag_get reads one IAM tag key definition by namespace + name.
package oracle_identity_tag_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Get Tag Key"
	Description  = "Fetch a single Oracle Cloud tag key definition by its namespace OCID and tag name — its description, retired flag, cost-tracking flag and lifecycle state."
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
	{Name: "tag_namespace_ocid", Type: core.ConnectionTypeString, Label: "Tag Namespace OCID", Placeholder: "ocid1.tagnamespace.oc1..aaaa… that contains the tag", Required: true},
	{Name: "tag_name", Type: core.ConnectionTypeString, Label: "Tag Name", Placeholder: "The tag key name, e.g. CostCenter", Required: true},
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
	resp, err := client.GetTag(iam.Context(), identity.GetTagRequest{TagNamespaceId: &namespaceID, TagName: &tagName})
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
	return iam.Result(fmt.Sprintf("Tag %q in namespace %q is %s", tag["name"], tag["tag_namespace_name"], tag["lifecycle_state"]), map[string]interface{}{"tag": tag, "id": tag["id"]}), nil
}
