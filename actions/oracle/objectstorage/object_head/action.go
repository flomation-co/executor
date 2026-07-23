// Package oracle_objectstorage_object_head fetches an object's metadata (size,
// content type, ETag) without downloading its content — useful to check whether
// an object exists or how big it is before fetching.
package oracle_objectstorage_object_head

import (
	"fmt"

	core "flomation.app/automate/executor"
	os "flomation.app/automate/executor/actions/oracle/objectstorage"

	ocios "github.com/oracle/oci-go-sdk/v65/objectstorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Object Storage: Head Object"
	Description  = "Fetch an object's metadata (size, content type, ETag, storage tier, archival state and any custom metadata) without downloading its content — use it to check whether an object exists, how big it is, or whether it must be restored from Archive first. The namespace is resolved automatically."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+circle-info"
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
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "Object Name", Placeholder: "The object key", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type"},
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag"},
	{Name: "storage_tier", Type: core.ConnectionTypeString, Label: "Storage Tier"},
	{Name: "archival_state", Type: core.ConnectionTypeString, Label: "Archival State"},
	{Name: "metadata", Type: core.ConnectionTypeObject, Label: "Custom Metadata"},
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
	resp, err := client.HeadObject(ctx, ocios.HeadObjectRequest{NamespaceName: &ns, BucketName: &bucket, ObjectName: &object})
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	var size int64
	if resp.ContentLength != nil {
		size = *resp.ContentLength
	}
	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Object %q is %d bytes (%s)", object, size, os.Str(resp.ContentType)),
		"size_bytes":     size,
		"content_type":   os.Str(resp.ContentType),
		"etag":           os.Str(resp.ETag),
		"storage_tier":   string(resp.StorageTier),
		"archival_state": string(resp.ArchivalState),
		"metadata":       resp.OpcMeta,
		"success":        true,
	}, nil
}
