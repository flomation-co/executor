// Package oracle_objectstorage_object_rename renames an object within a bucket (a
// synchronous server-side move — no re-upload).
package oracle_objectstorage_object_rename

import (
	"fmt"

	core "flomation.app/automate/executor"
	os "flomation.app/automate/executor/actions/oracle/objectstorage"

	ocios "github.com/oracle/oci-go-sdk/v65/objectstorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Object Storage: Rename Object"
	Description  = "Rename an object within an Oracle Cloud Object Storage bucket — a server-side move, no re-upload. By default the rename fails if an object already exists at the new name, so it can't silently overwrite an unrelated object; enable Overwrite to replace it. The namespace is resolved automatically."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+pen"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "The bucket", Required: true},
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "Current Object Name", Placeholder: "The object to rename", Required: true},
	{Name: "new_object_name", Type: core.ConnectionTypeString, Label: "New Object Name", Placeholder: "The new name/key", Required: true},
	{Name: "overwrite", Type: core.ConnectionTypeBoolean, Label: "Overwrite if exists", Placeholder: "If the new name already exists: off (default) fails without touching it, on replaces it"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "New Object Name"},
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := os.GetAuth(inputs)
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	bucket, err := os.RequiredString("bucket_name", inputs)
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	source, err := os.RequiredString("object_name", inputs)
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	newName, err := os.RequiredString("new_object_name", inputs)
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	client, err := auth.Client()
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := os.Context()
	ns, err := auth.Namespace(ctx, client)
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	overwrite := os.OptionalBool("overwrite", inputs, false)
	details := ocios.RenameObjectDetails{SourceName: &source, NewName: &newName}
	if !overwrite {
		// "*" makes OCI fail the rename if an object already exists at NewName,
		// so a rename can't silently destroy an unrelated object.
		details.NewObjIfNoneMatchETag = os.StringPtr("*")
	}
	resp, err := client.RenameObject(ctx, ocios.RenameObjectRequest{
		NamespaceName:       &ns,
		BucketName:          &bucket,
		RenameObjectDetails: details,
	})
	if err != nil {
		// Our fail-safe guard tripped: OCI returns IfNoneMatchFailed / HTTP 412
		// because an object already exists at the new name. Turn that raw code
		// into something the operator can act on.
		if !overwrite {
			if code, status := os.ServiceErrorCode(err); code == "IfNoneMatchFailed" || status == 412 {
				return os.ErrorResult(fmt.Sprintf("an object named %q already exists in bucket %q — enable Overwrite to replace it", newName, bucket)), nil
			}
		}
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Renamed %q → %q in bucket %q", source, newName, bucket),
		"object_name": newName,
		"etag":        os.Str(resp.ETag),
		"success":     true,
	}, nil
}
