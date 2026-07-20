package azure_storage_container_get

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Get Container"
	Description  = "Fetch a container's properties and metadata (lease state, public access level, immutability flags)"
	Website      = "https://www.flomation.co"
	Icon         = "azure+magnifying-glass"
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
	{Name: "container", Type: core.ConnectionTypeString, Label: "Container Name", Placeholder: "my-container", Required: true},
	{Name: "lease_id", Type: core.ConnectionTypeString, Label: "Lease ID", Placeholder: "Only needed when the blob or container is leased — the Lease ID output of a Lease step"},
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

	cc, err := auth.ContainerClient(containerName)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	// A lease_id is accepted here as an assertion ("fail unless this lease is
	// active and mine"), exactly as the header path used to send it on GET.
	opts := &container.GetPropertiesOptions{}
	if lid := storage.LeaseIDPtr(inputs); lid != nil {
		opts.LeaseAccessConditions = &container.LeaseAccessConditions{LeaseID: lid}
	}

	resp, err := cc.GetProperties(flow.GoContext(), opts)
	if err != nil {
		if storage.HasCode(err, bloberror.ContainerNotFound) {
			return storage.ErrorResult(fmt.Sprintf("container %q does not exist", containerName)), nil
		}
		_, msg := auth.SDKError(err)
		return storage.ErrorResult(msg), nil
	}

	// The SDK exposes as typed fields what the header path camelCased out of the
	// x-ms-* response headers; rebuild the same {properties, metadata} envelope.
	props := map[string]interface{}{}
	if resp.ETag != nil {
		props["etag"] = string(*resp.ETag)
	}
	if resp.LastModified != nil {
		props["lastModified"] = resp.LastModified.UTC().Format(time.RFC1123)
	}
	if resp.BlobPublicAccess != nil {
		props["blobPublicAccess"] = string(*resp.BlobPublicAccess)
	}
	if resp.LeaseState != nil {
		props["leaseState"] = string(*resp.LeaseState)
	}
	if resp.LeaseStatus != nil {
		props["leaseStatus"] = string(*resp.LeaseStatus)
	}
	if resp.LeaseDuration != nil {
		props["leaseDuration"] = string(*resp.LeaseDuration)
	}
	if resp.HasImmutabilityPolicy != nil {
		props["hasImmutabilityPolicy"] = *resp.HasImmutabilityPolicy
	}
	if resp.HasLegalHold != nil {
		props["hasLegalHold"] = *resp.HasLegalHold
	}

	return storage.PropsResult(containerName, fmt.Sprintf("Fetched container %s", containerName),
		props, storage.StrMeta(resp.Metadata)), nil
}
