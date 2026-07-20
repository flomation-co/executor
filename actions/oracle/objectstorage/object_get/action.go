// Package oracle_objectstorage_object_get downloads an object's content from an
// Object Storage bucket. The content is returned as a string, plus a base64
// encoding so binary objects survive round-tripping through a flow.
package oracle_objectstorage_object_get

import (
	"encoding/base64"
	"fmt"
	"io"

	core "flomation.app/automate/executor"
	os "flomation.app/automate/executor/actions/oracle/objectstorage"

	ocios "github.com/oracle/oci-go-sdk/v65/objectstorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Object Storage: Get Object"
	Description  = "Download an object's content from an Oracle Cloud Object Storage bucket. Returns the content as text plus a base64 encoding (use the base64 output for binary objects). The namespace is resolved automatically."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+cloud-arrow-down"
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
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "The source bucket", Required: true},
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "Object Name", Placeholder: "The object key to download", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Content (text)"},
	{Name: "content_base64", Type: core.ConnectionTypeString, Label: "Content (base64)"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
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
	resp, err := client.GetObject(ctx, ocios.GetObjectRequest{NamespaceName: &ns, BucketName: &bucket, ObjectName: &object})
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	var data []byte
	if resp.Content != nil {
		data, err = io.ReadAll(resp.Content)
		_ = resp.Content.Close()
		if err != nil {
			return os.ErrorResult(auth.OCIError(err)), nil
		}
	}
	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Downloaded %q (%d bytes) from bucket %q", object, len(data), bucket),
		"content":        string(data),
		"content_base64": base64.StdEncoding.EncodeToString(data),
		"content_type":   os.Str(resp.ContentType),
		"size_bytes":     len(data),
		"etag":           os.Str(resp.ETag),
		"storage_tier":   string(resp.StorageTier),
		"archival_state": string(resp.ArchivalState),
		"metadata":       resp.OpcMeta,
		"success":        true,
	}, nil
}
