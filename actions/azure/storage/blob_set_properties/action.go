package azure_storage_blob_set_properties

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"
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

	// Set Blob Properties REPLACES the whole x-ms-blob-content-* set: a slot
	// omitted from the request is cleared on the blob. There is no read-
	// modify-write here on purpose — the Description warns instead, and the
	// operator sends every property they want kept.
	headers := map[string]string{}
	for input, header := range map[string]string{
		"content_type":        "x-ms-blob-content-type",
		"cache_control":       "x-ms-blob-cache-control",
		"content_disposition": "x-ms-blob-content-disposition",
		"content_encoding":    "x-ms-blob-content-encoding",
		"content_language":    "x-ms-blob-content-language",
	} {
		if v := storage.OptionalString(input, inputs); v != "" {
			headers[header] = v
		}
	}
	if len(headers) == 0 {
		return storage.ErrorResult("set at least one property (content_type, cache_control, content_disposition, content_encoding, content_language)"), nil
	}
	// Counted BEFORE the lease header joins them: x-ms-lease-id proves the
	// caller holds the lock, it is not a property being set, and the summary
	// would otherwise claim one more property than the operator asked for.
	propertyCount := len(headers)
	headers = storage.LeaseHeader(headers, inputs)

	resp, err := storage.Do(flow, auth, storage.Request{
		Method:  http.MethodPut,
		Path:    storage.BlobPath(container, blobName),
		Query:   url.Values{"comp": []string{"properties"}},
		Headers: headers,
	})
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	if err := storage.CheckResponse(resp); err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	return storage.ResourceResult(blobName, storage.HeadersResult(blobName, resp.Headers),
		fmt.Sprintf("Set %d properties on %s", propertyCount, blobName)), nil
}
