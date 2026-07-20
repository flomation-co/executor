package azure_storage_blob_snapshot

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Snapshot Blob"
	Description  = "Create a read-only point-in-time snapshot of a blob. The returned snapshot ID (a timestamp) addresses it later via ?snapshot="
	Website      = "https://www.flomation.co"
	Icon         = "azure+clock-rotate-left"
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
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Blob Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Snapshot"},
	{Name: "snapshot", Type: core.ConnectionTypeString, Label: "Snapshot ID"},
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

	bc, err := auth.BlobClient(container, blobName)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	// This action takes no metadata or lease inputs, so the snapshot inherits
	// the base blob's metadata and is created unconditionally — a nil options
	// mirrors the bare PUT ?comp=snapshot the REST path sent.
	resp, err := bc.CreateSnapshot(flow.GoContext(), nil)
	if err != nil {
		if storage.HasCode(err, bloberror.BlobNotFound) {
			return storage.ErrorResult(fmt.Sprintf("blob %q was not found in container %q", blobName, container)), nil
		}
		_, msg := auth.SDKError(err)
		return storage.ErrorResult(msg), nil
	}

	snapshot := ""
	if resp.Snapshot != nil {
		snapshot = *resp.Snapshot
	}

	// Build the {name, properties} envelope directly (not via WriteResult, which
	// would add a top-level etag key this action never exposed) so the top-level
	// output keys stay exactly id/result/snapshot/tool_result/success/error.
	props := map[string]interface{}{}
	if resp.ETag != nil {
		props["etag"] = string(*resp.ETag)
	}
	if resp.LastModified != nil {
		props["lastModified"] = resp.LastModified.UTC().Format(time.RFC1123)
	}

	out := storage.ResourceResult(blobName, map[string]interface{}{"name": blobName, "properties": props},
		fmt.Sprintf("Created snapshot %s of %s", snapshot, blobName))
	out["snapshot"] = snapshot
	return out, nil
}
