// Package oracle_identity_tag_namespace_get reads one IAM tag namespace by OCID.
package oracle_identity_tag_namespace_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Get Tag Namespace"
	Description  = "Fetch a single Oracle Cloud tag namespace by OCID — its name, description, compartment, retired flag and lifecycle state."
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
	{Name: "tag_namespace_ocid", Type: core.ConnectionTypeString, Label: "Tag Namespace OCID", Placeholder: "ocid1.tagnamespace.oc1..aaaa… of the namespace to fetch", Required: true},
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
	resp, err := client.GetTagNamespace(iam.Context(), identity.GetTagNamespaceRequest{TagNamespaceId: &id})
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
		"freeform_tags":   ns.FreeformTags,
		"defined_tags":    ns.DefinedTags,
		"time_created":    iam.FormatTime(ns.TimeCreated),
	}
	return iam.Result(fmt.Sprintf("Tag namespace %q is %s", tagNamespace["name"], tagNamespace["lifecycle_state"]), map[string]interface{}{"tag_namespace": tagNamespace, "id": tagNamespace["id"]}), nil
}
