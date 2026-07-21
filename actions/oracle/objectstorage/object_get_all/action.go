// Package oracle_objectstorage_object_get_all lists the objects in an Object
// Storage bucket, optionally filtered by name prefix.
package oracle_objectstorage_object_get_all

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	os "flomation.app/automate/executor/actions/oracle/objectstorage"

	ocios "github.com/oracle/oci-go-sdk/v65/objectstorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Object Storage: List Objects"
	Description  = "List the objects in an Oracle Cloud Object Storage bucket, with their size, creation time and last-modified time. Optionally filter by name prefix (e.g. a folder). The namespace is resolved automatically. Large buckets are capped — check the 'truncated' output to tell a complete listing from a partial one."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
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
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "The bucket to list", Required: true},
	{Name: "prefix", Type: core.ConnectionTypeString, Label: "Name Prefix", Placeholder: "Only objects whose name starts with this, e.g. reports/ (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "objects", Type: core.ConnectionTypeObject, Label: "Objects"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
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
	client, err := auth.Client()
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := os.Context()
	ns, err := auth.Namespace(ctx, client)
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}

	req := ocios.ListObjectsRequest{NamespaceName: &ns, BucketName: &bucket, Fields: os.StringPtr("size,timeCreated,timeModified,etag")}
	if p := strings.TrimSpace(os.OptionalString("prefix", inputs)); p != "" {
		req.Prefix = &p
	}
	var objects []map[string]interface{}
	truncated := false
	for page := 0; page < os.ListMaxPages; page++ {
		resp, err := client.ListObjects(ctx, req)
		if err != nil {
			return os.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Objects {
			o := &resp.Objects[i]
			m := map[string]interface{}{"name": os.Str(o.Name)}
			if o.Size != nil {
				m["size_bytes"] = *o.Size
			}
			if o.TimeCreated != nil {
				m["time_created"] = os.FormatTime(o.TimeCreated)
			}
			if o.TimeModified != nil {
				m["time_modified"] = os.FormatTime(o.TimeModified)
			}
			m["etag"] = os.Str(o.Etag)
			objects = append(objects, m)
		}
		if resp.NextStartWith == nil || *resp.NextStartWith == "" {
			break
		}
		req.Start = resp.NextStartWith
		if page == os.ListMaxPages-1 {
			// Hit the page cap with more objects still available.
			truncated = true
		}
	}

	summary := fmt.Sprintf("Found %d object(s) in bucket %q", len(objects), bucket)
	if truncated {
		summary = fmt.Sprintf("Found at least %d object(s) in bucket %q (list truncated at %d pages — more available)", len(objects), bucket, os.ListMaxPages)
	}
	return map[string]interface{}{
		"tool_result": summary,
		"objects":     objects,
		"count":       len(objects),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
