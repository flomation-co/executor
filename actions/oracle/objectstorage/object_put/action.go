// Package oracle_objectstorage_object_put uploads an object (its content) into an
// Object Storage bucket. Content is taken as a string; enable "base64" to upload
// binary content that's been base64-encoded upstream.
package oracle_objectstorage_object_put

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	core "flomation.app/automate/executor"
	os "flomation.app/automate/executor/actions/oracle/objectstorage"

	ocios "github.com/oracle/oci-go-sdk/v65/objectstorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Object Storage: Put Object"
	Description  = "Upload an object into an Oracle Cloud Object Storage bucket. Content is a string (text or upstream data); enable Base64 to upload binary content that was base64-encoded upstream. The namespace is resolved automatically."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+cloud-arrow-up"
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
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "The target bucket", Required: true},
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "Object Name", Placeholder: "The object key, e.g. reports/2026-07.csv", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Content", Placeholder: "The object's contents (text, or base64 if Base64 is on)"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type", Placeholder: "e.g. text/plain, application/json (optional)"},
	{Name: "metadata", Type: core.ConnectionTypeString, Label: "Custom Metadata (JSON)", Placeholder: `{"source":"crm"} — stored as opc-meta-* headers (optional)`},
	{Name: "base64", Type: core.ConnectionTypeBoolean, Label: "Content is Base64", Placeholder: "Decode the content from base64 before upload (for binary data)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "Object Name"},
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
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

	body := []byte(os.OptionalString("content", inputs))
	if os.OptionalBool("base64", inputs, false) {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(body)))
		if err != nil {
			return os.ErrorResult("content is not valid base64 — turn off Base64, or supply base64-encoded content"), nil
		}
		body = decoded
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

	meta, err := os.StringMap("metadata", "metadata", inputs)
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}

	length := int64(len(body))
	req := ocios.PutObjectRequest{
		NamespaceName: &ns,
		BucketName:    &bucket,
		ObjectName:    &object,
		ContentLength: &length,
		PutObjectBody: io.NopCloser(strings.NewReader(string(body))),
	}
	if ct := strings.TrimSpace(os.OptionalString("content_type", inputs)); ct != "" {
		req.ContentType = &ct
	}
	if len(meta) > 0 {
		req.OpcMeta = meta
	}
	resp, err := client.PutObject(ctx, req)
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Uploaded %q (%d bytes) to bucket %q", object, length, bucket),
		"object_name": object,
		"etag":        os.Str(resp.ETag),
		"size_bytes":  length,
		"success":     true,
	}, nil
}
