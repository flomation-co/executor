// Package oracle_objectstorage_object_delete permanently deletes an object from an
// Object Storage bucket.
package oracle_objectstorage_object_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	os "flomation.app/automate/executor/actions/oracle/objectstorage"

	ocios "github.com/oracle/oci-go-sdk/v65/objectstorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Object Storage: Delete Object"
	Description  = "Permanently delete an object from an Oracle Cloud Object Storage bucket. The namespace is resolved automatically."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+trash"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "The bucket", Required: true},
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "Object Name", Placeholder: "The object key to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "Object Name"},
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
	object, err := os.RequiredString("object_name", inputs)
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
	if _, err := client.DeleteObject(ctx, ocios.DeleteObjectRequest{NamespaceName: &ns, BucketName: &bucket, ObjectName: &object}); err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted object %q from bucket %q", object, bucket),
		"object_name": object,
		"success":     true,
	}, nil
}
