// Package oracle_objectstorage_presigned_url_create creates a pre-authenticated
// request (PAR) for an object — a time-limited URL that grants read (download) or
// write (upload) access without OCI credentials, the OCI equivalent of an S3
// presigned URL.
package oracle_objectstorage_presigned_url_create

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	os "flomation.app/automate/executor/actions/oracle/objectstorage"

	"github.com/oracle/oci-go-sdk/v65/common"
	ocios "github.com/oracle/oci-go-sdk/v65/objectstorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Object Storage: Create Presigned URL"
	Description  = "Create a pre-authenticated request (PAR) for an object — a time-limited URL granting read (download) or write (upload) access with no credentials, like an S3 presigned URL. The namespace is resolved automatically."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+link"
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
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "The bucket the object is in", Required: true},
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "Object Name", Placeholder: "The object to grant access to", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "PAR Name", Placeholder: "A label for this pre-authenticated request", Required: true},
	{Name: "access_type", Type: core.ConnectionTypeString, Label: "Access", Placeholder: "What the URL allows", Required: true, Options: []core.ConnectionOption{
		{Name: "Read (download)", Value: "ObjectRead"},
		{Name: "Write (upload)", Value: "ObjectWrite"},
		{Name: "Read & write", Value: "ObjectReadWrite"},
	}},
	{Name: "expires_in_hours", Type: core.ConnectionTypeInteger, Label: "Expires in (hours)", Placeholder: "How long the URL is valid — default 24"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Presigned URL"},
	{Name: "access_uri", Type: core.ConnectionTypeString, Label: "Access URI (path)"},
	{Name: "par_id", Type: core.ConnectionTypeString, Label: "PAR ID"},
	{Name: "expires_at", Type: core.ConnectionTypeString, Label: "Expires At"},
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
	parName, err := os.RequiredString("name", inputs)
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	accessType, err := os.RequiredString("access_type", inputs)
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	hours, err := os.OptionalInt("expires_in_hours", inputs, 24)
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	if hours <= 0 {
		return os.ErrorResult("expires in (hours) must be greater than 0"), nil
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

	expires := time.Now().Add(time.Duration(hours) * time.Hour)
	details := ocios.CreatePreauthenticatedRequestDetails{
		Name:        &parName,
		ObjectName:  &object,
		AccessType:  ocios.CreatePreauthenticatedRequestDetailsAccessTypeEnum(accessType),
		TimeExpires: &common.SDKTime{Time: expires},
	}
	resp, err := client.CreatePreauthenticatedRequest(ctx, ocios.CreatePreauthenticatedRequestRequest{
		NamespaceName:                        &ns,
		BucketName:                           &bucket,
		CreatePreauthenticatedRequestDetails: details,
	})
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	accessURI := os.Str(resp.AccessUri)
	// Build the full URL from the client's own resolved endpoint (client.Host,
	// e.g. https://objectstorage.uk-london-1.oraclecloud.com) rather than a
	// hardcoded realm, so gov/EU-sovereign realms get the correct host.
	fullURL := strings.TrimRight(client.Host, "/") + accessURI
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created %s URL for %q, valid %dh", accessType, object, hours),
		"url":         fullURL,
		"access_uri":  accessURI,
		"par_id":      os.Str(resp.Id),
		"expires_at":  expires.UTC().Format(time.RFC3339),
		"success":     true,
	}, nil
}
