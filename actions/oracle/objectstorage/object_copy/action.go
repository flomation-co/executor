// Package oracle_objectstorage_object_copy copies an object to another name
// and/or bucket (and optionally another region). The copy runs asynchronously in
// OCI; the action returns the work-request id that tracks it.
package oracle_objectstorage_object_copy

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
	Name         = "OCI Object Storage: Copy Object"
	Description  = "Copy an object to another name and/or bucket (and optionally another region). The copy runs asynchronously; the work-request id is returned. Destination bucket and region default to the source. The namespace is resolved automatically."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+copy"
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
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Source Bucket", Placeholder: "The bucket the object is in", Required: true},
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "Source Object Name", Placeholder: "The object to copy", Required: true},
	{Name: "destination_object_name", Type: core.ConnectionTypeString, Label: "Destination Object Name", Placeholder: "The name for the copy", Required: true},
	{Name: "destination_bucket", Type: core.ConnectionTypeString, Label: "Destination Bucket", Placeholder: "Leave blank to copy within the source bucket"},
	{Name: "destination_region", Type: core.ConnectionTypeString, Label: "Destination Region", Placeholder: "Leave blank for the same region (e.g. uk-london-1)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request ID"},
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
	destObject, err := os.RequiredString("destination_object_name", inputs)
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

	destBucket := bucket
	if v := strings.TrimSpace(os.OptionalString("destination_bucket", inputs)); v != "" {
		destBucket = v
	}
	destRegion := auth.Region
	if v := strings.ToLower(strings.TrimSpace(os.OptionalString("destination_region", inputs))); v != "" {
		destRegion = v
	}
	details := ocios.CopyObjectDetails{
		SourceObjectName:      &source,
		DestinationRegion:     &destRegion,
		DestinationNamespace:  &ns,
		DestinationBucket:     &destBucket,
		DestinationObjectName: &destObject,
	}
	resp, err := client.CopyObject(ctx, ocios.CopyObjectRequest{NamespaceName: &ns, BucketName: &bucket, CopyObjectDetails: details})
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Copy of %q → %q in bucket %q started", source, destObject, destBucket),
		"work_request_id": os.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
