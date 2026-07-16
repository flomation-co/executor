package azure_storage_blob_set_tier

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
	Name         = "Azure Storage: Set Blob Tier"
	Description  = "Change a block blob's access tier (Hot/Cool/Cold/Archive). Moving OUT of Archive starts a rehydration that can take hours — High priority speeds it up"
	Website      = "https://www.flomation.co"
	Icon         = "azure+layer-group"
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
	{
		Name:     "access_tier",
		Type:     core.ConnectionTypeString,
		Label:    "Access Tier",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Hot", Value: "Hot"},
			{Name: "Cool", Value: "Cool"},
			{Name: "Cold", Value: "Cold"},
			{Name: "Archive", Value: "Archive"},
		},
	},
	{
		Name:  "rehydrate_priority",
		Type:  core.ConnectionTypeString,
		Label: "Rehydrate Priority",
		Options: []core.ConnectionOption{
			{Name: "Default", Value: ""},
			{Name: "Standard", Value: "Standard"},
			{Name: "High", Value: "High"},
		},
	},
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
	tier, err := storage.RequiredString("access_tier", inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	headers := map[string]string{"x-ms-access-tier": tier}
	if priority := storage.OptionalString("rehydrate_priority", inputs); priority != "" {
		headers["x-ms-rehydrate-priority"] = priority
	}
	headers = storage.LeaseHeader(headers, inputs)

	resp, err := storage.Do(flow, auth, storage.Request{
		Method:  http.MethodPut,
		Path:    storage.BlobPath(container, blobName),
		Query:   url.Values{"comp": []string{"tier"}},
		Headers: headers,
	})
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	if err := storage.CheckResponse(resp); err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	// 200 = tier changed immediately; 202 = archive rehydration accepted and
	// running in the background.
	summary := fmt.Sprintf("Set tier of %s to %s", blobName, tier)
	pending := resp.StatusCode == http.StatusAccepted
	if pending {
		summary = fmt.Sprintf("Rehydration of %s to %s accepted (runs in the background)", blobName, tier)
	}
	return storage.ResourceResult(blobName, map[string]interface{}{"accessTier": tier, "pending": pending}, summary), nil
}
