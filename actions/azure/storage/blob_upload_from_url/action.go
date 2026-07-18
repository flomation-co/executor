package azure_storage_blob_upload_from_url

import (
	"fmt"
	"net/url"
	"time"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Upload Blob from URL"
	Description  = "Create a block blob by having Azure fetch a publicly reachable URL server-side (synchronous, up to 256 MB). For larger or cross-account copies use Copy Blob"
	Website      = "https://www.flomation.co"
	Icon         = "azure+link"
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
	{Name: "blob_name", Type: core.ConnectionTypeString, Label: "Blob Name", Placeholder: "downloads/report.pdf", Required: true},
	{Name: "source_url", Type: core.ConnectionTypeString, Label: "Source URL", Placeholder: "https://example.com/file.pdf — must be reachable by Azure, or carry its own SAS", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Blob Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Blob"},
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
	source, err := storage.RequiredString("source_url", inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	if u, err := url.Parse(source); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return storage.ErrorResult("source_url must be an http(s) URL"), nil
	}

	// UploadBlobFromURL (Put Blob From URL) is the SYNCHRONOUS server-side
	// put-from-URL: Azure fetches the source and returns only when the block
	// blob is written, unlike Copy Blob's async start. Source-blob properties
	// are copied by default, which preserves the old header-echoed shape.
	bb, err := auth.BlockBlobClient(container, blobName)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	resp, err := bb.UploadBlobFromURL(flow.GoContext(), source, nil)
	if err != nil {
		// The source URL may carry a SAS; the friendly message names no URL and
		// SDKError redacts sig= from anything that does.
		if storage.HasCode(err, bloberror.CannotVerifyCopySource) {
			return storage.ErrorResult("Azure could not fetch the source URL (CannotVerifyCopySource): check it is publicly reachable or carries a valid SAS"), nil
		}
		_, msg := auth.SDKError(err)
		return storage.ErrorResult(msg), nil
	}

	props := map[string]interface{}{}
	if resp.ETag != nil {
		props["etag"] = string(*resp.ETag)
	}
	if resp.LastModified != nil {
		props["lastModified"] = resp.LastModified.UTC().Format(time.RFC1123)
	}
	return storage.PropsResult(blobName, fmt.Sprintf("Created %s in %s from URL", blobName, container), props, nil), nil
}
