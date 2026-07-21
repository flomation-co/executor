// Package oracle_objectstorage_presigned_url_get_all lists the pre-authenticated
// requests (PARs) on a bucket — their name, target object, access type and
// expiry. (Listing does not return the URLs themselves; those are shown only at
// creation time.)
package oracle_objectstorage_presigned_url_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	os "flomation.app/automate/executor/actions/oracle/objectstorage"

	ocios "github.com/oracle/oci-go-sdk/v65/objectstorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Object Storage: List Presigned URLs"
	Description  = "List the pre-authenticated requests (PARs) on a bucket — their name, target object, access type and expiry. The URLs themselves are only shown at creation time. The namespace is resolved automatically."
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
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "The bucket", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "presigned_urls", Type: core.ConnectionTypeObject, Label: "Presigned URLs (PARs)"},
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

	var pars []map[string]interface{}
	req := ocios.ListPreauthenticatedRequestsRequest{NamespaceName: &ns, BucketName: &bucket}
	truncated := false
	for page := 0; page < os.ListMaxPages; page++ {
		resp, err := client.ListPreauthenticatedRequests(ctx, req)
		if err != nil {
			return os.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			p := &resp.Items[i]
			m := map[string]interface{}{
				"id":          os.Str(p.Id),
				"name":        os.Str(p.Name),
				"object_name": os.Str(p.ObjectName),
				"access_type": string(p.AccessType),
			}
			if p.TimeExpires != nil {
				m["expires_at"] = os.FormatTime(p.TimeExpires)
			}
			if p.TimeCreated != nil {
				m["time_created"] = os.FormatTime(p.TimeCreated)
			}
			pars = append(pars, m)
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
		if page == os.ListMaxPages-1 {
			truncated = true
		}
	}

	summary := fmt.Sprintf("Found %d presigned URL(s) on bucket %q", len(pars), bucket)
	if truncated {
		summary = fmt.Sprintf("Found at least %d presigned URL(s) on bucket %q (list truncated at %d pages — more available)", len(pars), bucket, os.ListMaxPages)
	}
	return map[string]interface{}{
		"tool_result":    summary,
		"presigned_urls": pars,
		"count":          len(pars),
		"truncated":      truncated,
		"success":        true,
	}, nil
}
