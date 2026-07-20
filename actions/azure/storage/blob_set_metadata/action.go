package azure_storage_blob_set_metadata

import (
	"fmt"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Set Blob Metadata"
	Description  = "Replace a blob's metadata without re-uploading its content. The metadata object supplied here becomes the blob's ENTIRE metadata set — keys not included are removed"
	Website      = "https://www.flomation.co"
	Icon         = "azure+pen"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "account_name", Type: core.ConnectionTypeString, Label: "Storage Account", Placeholder: "mystorageaccount", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Shared Key", Value: "shared_key"}, {Name: "Microsoft Entra (service principal)", Value: "entra"}}},
	{Name: "account_key", Type: core.ConnectionTypeSecret, Label: "Account Key", Placeholder: "Base64 account key — Storage Account ▸ Access keys", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "shared_key"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a Storage Blob Data role on the account (RBAC)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://myaccount.blob.core.windows.net — leave blank to derive; Azurite: http://host:10000/devstoreaccount1"},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip TLS verification — only for custom endpoints with a self-signed certificate"},
	{Name: "container", Type: core.ConnectionTypeString, Label: "Container", Placeholder: "my-container", Required: true},
	{Name: "blob_name", Type: core.ConnectionTypeString, Label: "Blob Name", Placeholder: "reports/2026/summary.pdf", Required: true},
	{Name: "metadata", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `{"reviewed":"true"} — replaces ALL existing metadata`, Required: true},
	{Name: "lease_id", Type: core.ConnectionTypeString, Label: "Lease ID", Placeholder: "Only needed when the blob or container is leased — the Lease ID output of a Lease step"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Blob Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := storage.GetAuth(inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	container, err := storage.RequiredString("container", inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	blobName, err := storage.RequiredString("blob_name", inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	// The supplied object becomes the blob's ENTIRE metadata set — SetMetadata
	// replaces, so keys not included here are removed. Required, and blank is
	// rejected up front (an empty set would silently wipe existing metadata).
	meta, err := storage.MetadataMap(inputs, "metadata")
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	if len(meta) == 0 {
		return storage.ErrorResult("metadata is required"), nil
	}

	bc, err := auth.BlobClient(container, blobName)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	opts := &blob.SetMetadataOptions{}
	if lid := storage.LeaseIDPtr(inputs); lid != nil {
		opts.AccessConditions = &blob.AccessConditions{LeaseAccessConditions: &blob.LeaseAccessConditions{LeaseID: lid}}
	}

	resp, err := bc.SetMetadata(flow.GoContext(), meta, opts)
	if err != nil {
		_, msg := auth.SDKError(err)
		return storage.ErrorResult(msg), nil
	}

	return storage.WriteResult(blobName, resp.ETag, resp.LastModified,
		fmt.Sprintf("Set metadata on %s", blobName)), nil
}
