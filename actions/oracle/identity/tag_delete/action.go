// Package oracle_identity_tag_delete deletes a tag key (a defined tag) from a tag namespace.
// Asynchronous — returns a work-request id.
package oracle_identity_tag_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Delete Tag Key"
	Description  = "Delete a tag key definition from an Oracle Cloud tag namespace, identified by the namespace OCID and the tag name. Asynchronous — returns a work-request id; the tag moves to DELETING then DELETED."
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
	{Name: "tag_namespace_ocid", Type: core.ConnectionTypeString, Label: "Tag Namespace OCID", Placeholder: "ocid1.tagnamespace.oc1..aaaa… containing the tag", Required: true},
	{Name: "tag_name", Type: core.ConnectionTypeString, Label: "Tag Name", Placeholder: "the tag key to delete, e.g. CostCentre", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Tag Namespace OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, namespaceID, errResult := iam.ResourceClient(inputs, "tag_namespace_ocid")
	if errResult != nil {
		return errResult, nil
	}
	tagName, err := iam.RequiredString("tag_name", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	resp, err := client.DeleteTag(iam.Context(), identity.DeleteTagRequest{TagNamespaceId: &namespaceID, TagName: &tagName})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	result := iam.AsyncResult(fmt.Sprintf("Delete requested for tag %q in namespace %s", tagName, namespaceID), iam.Str(resp.OpcWorkRequestId))
	result["id"] = namespaceID
	return result, nil
}
