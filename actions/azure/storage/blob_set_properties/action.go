package azure_storage_blob_set_properties

import (
	"fmt"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Set Blob Properties"
	Description  = "Change a blob's HTTP properties (content type, cache control, disposition, encoding, language) without re-uploading. CAUTION: Azure treats this as a replace — any of the five properties left blank here is CLEARED on the blob"
	Website      = "https://www.flomation.co"
	Icon         = "azure+gear"
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
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type", Placeholder: "application/pdf"},
	{Name: "cache_control", Type: core.ConnectionTypeString, Label: "Cache Control", Placeholder: "max-age=3600"},
	{Name: "content_disposition", Type: core.ConnectionTypeString, Label: "Content Disposition", Placeholder: `attachment; filename="summary.pdf"`},
	{Name: "content_encoding", Type: core.ConnectionTypeString, Label: "Content Encoding", Placeholder: "gzip"},
	{Name: "content_language", Type: core.ConnectionTypeString, Label: "Content Language", Placeholder: "en-GB"},
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

	// SetHTTPHeaders REPLACES the whole content-property set: a property left
	// blank here is cleared on the blob. There is no read-modify-write on
	// purpose — the Description warns instead, and the operator sends every
	// property they want kept. Only the five properties present are set; the
	// rest stay nil and are cleared. propertyCount tracks how many the operator
	// supplied so the summary matches — the lease ID is not a property.
	headers := blob.HTTPHeaders{}
	propertyCount := 0
	if v := storage.OptionalString("content_type", inputs); v != "" {
		headers.BlobContentType = &v
		propertyCount++
	}
	if v := storage.OptionalString("cache_control", inputs); v != "" {
		headers.BlobCacheControl = &v
		propertyCount++
	}
	if v := storage.OptionalString("content_disposition", inputs); v != "" {
		headers.BlobContentDisposition = &v
		propertyCount++
	}
	if v := storage.OptionalString("content_encoding", inputs); v != "" {
		headers.BlobContentEncoding = &v
		propertyCount++
	}
	if v := storage.OptionalString("content_language", inputs); v != "" {
		headers.BlobContentLanguage = &v
		propertyCount++
	}
	if propertyCount == 0 {
		return storage.ErrorResult("set at least one property (content_type, cache_control, content_disposition, content_encoding, content_language)"), nil
	}

	bc, err := auth.BlobClient(container, blobName)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	opts := &blob.SetHTTPHeadersOptions{}
	if lid := storage.LeaseIDPtr(inputs); lid != nil {
		opts.AccessConditions = &blob.AccessConditions{LeaseAccessConditions: &blob.LeaseAccessConditions{LeaseID: lid}}
	}

	resp, err := bc.SetHTTPHeaders(flow.GoContext(), headers, opts)
	if err != nil {
		_, msg := auth.SDKError(err)
		return storage.ErrorResult(msg), nil
	}

	return storage.WriteResult(blobName, resp.ETag, resp.LastModified,
		fmt.Sprintf("Set %d properties on %s", propertyCount, blobName)), nil
}
