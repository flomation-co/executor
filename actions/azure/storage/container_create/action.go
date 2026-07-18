package azure_storage_container_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Create Container"
	Description  = "Create a blob container in the storage account, optionally with a public access level and metadata"
	Website      = "https://www.flomation.co"
	Icon         = "azure+plus"
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
	{Name: "container", Type: core.ConnectionTypeString, Label: "Container Name", Placeholder: "my-container — 3-63 lowercase letters, digits and hyphens", Required: true},
	{
		Name:  "public_access",
		Type:  core.ConnectionTypeString,
		Label: "Public Access",
		Options: []core.ConnectionOption{
			{Name: "Private (default)", Value: ""},
			{Name: "Blob (anonymous read for blobs)", Value: "blob"},
			{Name: "Container (anonymous read and list)", Value: "container"},
		},
	},
	{Name: "metadata", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `{"project":"alpha"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Container Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Container"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := storage.GetAuth(inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	containerName, err := storage.RequiredString("container", inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	if err := storage.ValidateContainerName(containerName); err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	meta, err := storage.MetadataMap(inputs, "metadata")
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	cc, err := auth.ContainerClient(containerName)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	opts := &container.CreateOptions{Metadata: meta}
	if access := storage.OptionalString("public_access", inputs); access != "" {
		pa := container.PublicAccessType(access)
		opts.Access = &pa
	}

	resp, err := cc.Create(flow.GoContext(), opts)
	if err != nil {
		if storage.HasCode(err, bloberror.ContainerAlreadyExists) {
			return storage.ErrorResult(fmt.Sprintf("container %q already exists", containerName)), nil
		}
		_, msg := auth.SDKError(err)
		return storage.ErrorResult(msg), nil
	}

	return storage.WriteResult(containerName, resp.ETag, resp.LastModified,
		fmt.Sprintf("Created container %s", containerName)), nil
}
