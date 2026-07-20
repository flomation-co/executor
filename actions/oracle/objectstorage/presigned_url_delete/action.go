// Package oracle_objectstorage_presigned_url_delete revokes a pre-authenticated
// request (PAR) by its id, immediately invalidating the URL.
package oracle_objectstorage_presigned_url_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	os "flomation.app/automate/executor/actions/oracle/objectstorage"

	ocios "github.com/oracle/oci-go-sdk/v65/objectstorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Object Storage: Delete Presigned URL"
	Description  = "Revoke a pre-authenticated request (PAR) by its id, immediately invalidating the URL. The namespace is resolved automatically."
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
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "The bucket the PAR is on", Required: true},
	{Name: "par_id", Type: core.ConnectionTypeString, Label: "PAR ID", Placeholder: "The pre-authenticated request id (from List Presigned URLs)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "par_id", Type: core.ConnectionTypeString, Label: "PAR ID"},
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
	parID, err := os.RequiredString("par_id", inputs)
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
	if _, err := client.DeletePreauthenticatedRequest(ctx, ocios.DeletePreauthenticatedRequestRequest{NamespaceName: &ns, BucketName: &bucket, ParId: &parID}); err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Revoked presigned URL %s on bucket %q", parID, bucket),
		"par_id":      parID,
		"success":     true,
	}, nil
}
