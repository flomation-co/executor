package azure_storage_blob_get_properties

import (
	"encoding/base64"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Get Blob Properties"
	Description  = "Fetch a blob's properties and metadata (size, tier, lease, copy status) via HEAD — without downloading its content"
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
	{Name: "container", Type: core.ConnectionTypeString, Label: "Container", Placeholder: "my-container", Required: true},
	{Name: "blob_name", Type: core.ConnectionTypeString, Label: "Blob Name", Placeholder: "reports/2026/summary.pdf", Required: true},
	{Name: "lease_id", Type: core.ConnectionTypeString, Label: "Lease ID", Placeholder: "Only needed when the blob or container is leased — the Lease ID output of a Lease step"},
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

	bc, err := auth.BlobClient(container, blobName)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	// GetProperties is a HEAD under the hood — it returns everything in typed
	// header fields without downloading the body. The lease_id (when set) rides
	// as a lease access-condition, the SDK-native form of the x-ms-lease-id the
	// REST path sent as an assertion.
	opts := &blob.GetPropertiesOptions{}
	if lid := storage.LeaseIDPtr(inputs); lid != nil {
		opts.AccessConditions = &blob.AccessConditions{LeaseAccessConditions: &blob.LeaseAccessConditions{LeaseID: lid}}
	}

	resp, err := bc.GetProperties(flow.GoContext(), opts)
	if err != nil {
		if storage.HasCode(err, bloberror.BlobNotFound) {
			return storage.ErrorResult(fmt.Sprintf("blob %q was not found in container %q", blobName, container)), nil
		}
		_, msg := auth.SDKError(err)
		return storage.ErrorResult(msg), nil
	}

	// Rebuild the {properties} map from the SDK's typed fields, using the same
	// camelCase keys the header path produced — the result object is opaque, so
	// only the top-level output surface must stay identical.
	props := map[string]interface{}{}
	if resp.ETag != nil {
		props["etag"] = string(*resp.ETag)
	}
	if resp.LastModified != nil {
		props["lastModified"] = resp.LastModified.UTC().Format(time.RFC1123)
	}
	if resp.CreationTime != nil {
		props["creationTime"] = resp.CreationTime.UTC().Format(time.RFC1123)
	}
	if resp.ContentLength != nil {
		props["contentLength"] = *resp.ContentLength
	}
	if resp.ContentType != nil {
		props["contentType"] = *resp.ContentType
	}
	if resp.ContentEncoding != nil {
		props["contentEncoding"] = *resp.ContentEncoding
	}
	if resp.ContentLanguage != nil {
		props["contentLanguage"] = *resp.ContentLanguage
	}
	if resp.ContentDisposition != nil {
		props["contentDisposition"] = *resp.ContentDisposition
	}
	if resp.CacheControl != nil {
		props["cacheControl"] = *resp.CacheControl
	}
	if len(resp.ContentMD5) > 0 {
		props["contentMD5"] = base64.StdEncoding.EncodeToString(resp.ContentMD5)
	}
	if resp.BlobType != nil {
		props["blobType"] = string(*resp.BlobType)
	}
	if resp.AccessTier != nil {
		props["accessTier"] = *resp.AccessTier
	}
	if resp.AccessTierInferred != nil {
		props["accessTierInferred"] = *resp.AccessTierInferred
	}
	if resp.ArchiveStatus != nil {
		props["archiveStatus"] = *resp.ArchiveStatus
	}
	if resp.LeaseStatus != nil {
		props["leaseStatus"] = string(*resp.LeaseStatus)
	}
	if resp.LeaseState != nil {
		props["leaseState"] = string(*resp.LeaseState)
	}
	if resp.LeaseDuration != nil {
		props["leaseDuration"] = string(*resp.LeaseDuration)
	}
	if resp.IsServerEncrypted != nil {
		props["serverEncrypted"] = *resp.IsServerEncrypted
	}
	if resp.VersionID != nil {
		props["versionId"] = *resp.VersionID
	}
	if resp.IsCurrentVersion != nil {
		props["isCurrentVersion"] = *resp.IsCurrentVersion
	}
	if resp.TagCount != nil {
		props["tagCount"] = *resp.TagCount
	}
	if resp.CopyStatus != nil {
		props["copyStatus"] = string(*resp.CopyStatus)
	}
	if resp.CopyID != nil {
		props["copyId"] = *resp.CopyID
	}
	if resp.CopyProgress != nil {
		props["copyProgress"] = *resp.CopyProgress
	}
	if resp.CopySource != nil {
		props["copySource"] = *resp.CopySource
	}
	if resp.CopyCompletionTime != nil {
		props["copyCompletionTime"] = resp.CopyCompletionTime.UTC().Format(time.RFC1123)
	}
	if resp.CopyStatusDescription != nil {
		props["copyStatusDescription"] = *resp.CopyStatusDescription
	}

	return storage.PropsResult(blobName, fmt.Sprintf("Fetched properties of %s", blobName),
		props, storage.StrMeta(resp.Metadata)), nil
}
