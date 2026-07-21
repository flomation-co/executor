// Package oracle_objectstorage_bucket_create creates an Object Storage bucket in
// an OCI compartment. The tenancy namespace is resolved automatically.
package oracle_objectstorage_bucket_create

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
	Name         = "OCI Object Storage: Create Bucket"
	Description  = "Create an Oracle Cloud Object Storage bucket in a compartment. The tenancy namespace is resolved automatically; optionally set the public-access type, storage tier and tags."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+plus"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "A unique bucket name within the namespace", Required: true},
	{Name: "public_access_type", Type: core.ConnectionTypeString, Label: "Public Access", Placeholder: "Bucket visibility", Options: []core.ConnectionOption{
		{Name: "None (private)", Value: "NoPublicAccess"},
		{Name: "Read objects", Value: "ObjectRead"},
		{Name: "Read objects & list", Value: "ObjectReadWithoutList"},
	}},
	{Name: "storage_tier", Type: core.ConnectionTypeString, Label: "Storage Tier", Placeholder: "Default is Standard", Options: []core.ConnectionOption{
		{Name: "Standard", Value: "Standard"},
		{Name: "Archive", Value: "Archive"},
	}},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "bucket", Type: core.ConnectionTypeObject, Label: "Bucket"},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := os.GetAuth(inputs)
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	name, err := os.RequiredString("bucket_name", inputs)
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

	details := ocios.CreateBucketDetails{Name: &name, CompartmentId: &compartment}
	if v := strings.TrimSpace(os.OptionalString("public_access_type", inputs)); v != "" {
		details.PublicAccessType = ocios.CreateBucketDetailsPublicAccessTypeEnum(v)
	}
	if v := strings.TrimSpace(os.OptionalString("storage_tier", inputs)); v != "" {
		details.StorageTier = ocios.CreateBucketDetailsStorageTierEnum(v)
	}
	tags, err := os.FreeformTags("tags", inputs)
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	details.FreeformTags = tags

	resp, err := client.CreateBucket(ctx, ocios.CreateBucketRequest{NamespaceName: &ns, CreateBucketDetails: details})
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created bucket %q in namespace %s", name, ns),
		"bucket": map[string]interface{}{
			"name":               os.Str(resp.Name),
			"namespace":          os.Str(resp.Namespace),
			"compartment_id":     os.Str(resp.CompartmentId),
			"public_access_type": string(resp.PublicAccessType),
			"storage_tier":       string(resp.StorageTier),
		},
		"namespace": ns,
		"success":   true,
	}, nil
}
