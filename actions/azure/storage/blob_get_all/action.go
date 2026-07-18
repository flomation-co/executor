package azure_storage_blob_get_all

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: List Blobs"
	Description  = "List the blobs in a container, optionally filtered by a name prefix and enriched with metadata, snapshots, versions, soft-deleted blobs, or tags"
	Website      = "https://www.flomation.co"
	Icon         = "azure+list"
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
	{Name: "prefix", Type: core.ConnectionTypeString, Label: "Prefix", Placeholder: "Only blobs whose name starts with this, e.g. reports/2026/"},
	{
		Name:        "include",
		Type:        core.ConnectionTypeComboBox,
		Label:       "Include",
		Placeholder: "Leave blank for nothing extra; combine with commas, e.g. metadata,tags",
		Options: []core.ConnectionOption{
			{Name: "Metadata", Value: "metadata"},
			{Name: "Index tags", Value: "tags"},
			{Name: "Metadata + index tags", Value: "metadata,tags"},
			{Name: "Snapshots", Value: "snapshots"},
			{Name: "Versions", Value: "versions"},
			{Name: "Soft-deleted blobs", Value: "deleted"},
			{Name: "Soft-deleted blobs with versions", Value: "deletedwithversions"},
			{Name: "Uncommitted blocks (failed/abandoned uploads)", Value: "uncommittedblobs"},
			{Name: "Copy state", Value: "copy"},
			{Name: "Permissions (hierarchical namespace)", Value: "permissions"},
			{Name: "Immutability policy", Value: "immutabilitypolicy"},
			{Name: "Legal hold", Value: "legalhold"},
		},
	},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow pagination until every blob is fetched"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max blobs to return when not returning all (default 50, max 5000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Blobs"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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

	returnAll := storage.OptionalBool("return_all", inputs)
	limit := storage.ClampLimit(storage.OptionalInt("limit", inputs))

	// return_all raises the page size to the 5000 maximum to minimise round
	// trips; otherwise limit is the single page's maxresults.
	pageSize := int32(limit)
	if returnAll {
		pageSize = int32(storage.MaxPageLimit)
	}

	opts := &container.ListBlobsFlatOptions{MaxResults: &pageSize}
	if prefix := storage.OptionalString("prefix", inputs); prefix != "" {
		opts.Prefix = &prefix
	}
	// ParseIncludeTokens keeps the same validation/error contract (unknown token
	// rejected); its comma-joined output maps onto the SDK's boolean Include set.
	includeStr, err := storage.ParseIncludeTokens(storage.OptionalString("include", inputs), storage.BlobIncludeTokens)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	if includeStr != "" {
		var inc container.ListBlobsInclude
		for _, tok := range strings.Split(includeStr, ",") {
			switch tok {
			case "copy":
				inc.Copy = true
			case "deleted":
				inc.Deleted = true
			case "deletedwithversions":
				inc.DeletedWithVersions = true
			case "immutabilitypolicy":
				inc.ImmutabilityPolicy = true
			case "legalhold":
				inc.LegalHold = true
			case "metadata":
				inc.Metadata = true
			case "permissions":
				inc.Permissions = true
			case "snapshots":
				inc.Snapshots = true
			case "tags":
				inc.Tags = true
			case "uncommittedblobs":
				inc.UncommittedBlobs = true
			case "versions":
				inc.Versions = true
			}
		}
		opts.Include = inc
	}

	cc, err := auth.ContainerClient(containerName)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	// blobItemMap reproduces BlobMap's output shape from the SDK's typed
	// container.BlobItem: name, the camelCased properties object, and the
	// snapshot/version/deleted/metadata/tags keys — each emitted only when the
	// service actually returned it (a requested include or a soft-deleted entry).
	blobItemMap := func(item *container.BlobItem) map[string]interface{} {
		out := map[string]interface{}{}
		if item.Name != nil {
			out["name"] = *item.Name
		}
		if item.Snapshot != nil && *item.Snapshot != "" {
			out["snapshot"] = *item.Snapshot
		}
		if item.VersionID != nil && *item.VersionID != "" {
			out["versionId"] = *item.VersionID
		}
		if item.IsCurrentVersion != nil {
			out["isCurrentVersion"] = *item.IsCurrentVersion
		}
		if item.Deleted != nil && *item.Deleted {
			out["deleted"] = true
		}
		if p := item.Properties; p != nil {
			props := map[string]interface{}{}
			if p.ETag != nil {
				props["etag"] = string(*p.ETag)
			}
			if p.LastModified != nil {
				props["lastModified"] = p.LastModified.UTC().Format(time.RFC1123)
			}
			if p.CreationTime != nil {
				props["creationTime"] = p.CreationTime.UTC().Format(time.RFC1123)
			}
			if p.ContentLength != nil {
				props["contentLength"] = *p.ContentLength
			}
			if p.ContentType != nil {
				props["contentType"] = *p.ContentType
			}
			if p.ContentEncoding != nil {
				props["contentEncoding"] = *p.ContentEncoding
			}
			if p.ContentLanguage != nil {
				props["contentLanguage"] = *p.ContentLanguage
			}
			if p.ContentDisposition != nil {
				props["contentDisposition"] = *p.ContentDisposition
			}
			if p.CacheControl != nil {
				props["cacheControl"] = *p.CacheControl
			}
			if len(p.ContentMD5) > 0 {
				props["contentMD5"] = base64.StdEncoding.EncodeToString(p.ContentMD5)
			}
			if p.BlobType != nil {
				props["blobType"] = string(*p.BlobType)
			}
			if p.AccessTier != nil {
				props["accessTier"] = string(*p.AccessTier)
			}
			if p.AccessTierInferred != nil {
				props["accessTierInferred"] = *p.AccessTierInferred
			}
			if p.ArchiveStatus != nil {
				props["archiveStatus"] = string(*p.ArchiveStatus)
			}
			if p.LeaseStatus != nil {
				props["leaseStatus"] = string(*p.LeaseStatus)
			}
			if p.LeaseState != nil {
				props["leaseState"] = string(*p.LeaseState)
			}
			if p.LeaseDuration != nil {
				props["leaseDuration"] = string(*p.LeaseDuration)
			}
			if p.ServerEncrypted != nil {
				props["serverEncrypted"] = *p.ServerEncrypted
			}
			if p.TagCount != nil {
				props["tagCount"] = *p.TagCount
			}
			if p.CopyStatus != nil {
				props["copyStatus"] = string(*p.CopyStatus)
			}
			if p.CopyID != nil {
				props["copyId"] = *p.CopyID
			}
			if p.CopyProgress != nil {
				props["copyProgress"] = *p.CopyProgress
			}
			if p.CopySource != nil {
				props["copySource"] = *p.CopySource
			}
			if p.CopyCompletionTime != nil {
				props["copyCompletionTime"] = p.CopyCompletionTime.UTC().Format(time.RFC1123)
			}
			if p.CopyStatusDescription != nil {
				props["copyStatusDescription"] = *p.CopyStatusDescription
			}
			if len(props) > 0 {
				out["properties"] = props
			}
		}
		if meta := storage.StrMeta(item.Metadata); len(meta) > 0 {
			out["metadata"] = meta
		}
		if item.BlobTags != nil {
			tags := map[string]interface{}{}
			for _, t := range item.BlobTags.BlobTagSet {
				if t == nil || t.Key == nil {
					continue
				}
				v := ""
				if t.Value != nil {
					v = *t.Value
				}
				tags[*t.Key] = v
			}
			out["tags"] = tags
		}
		return out
	}

	// maxListPages bounds a return_all walk exactly as the REST helper did — at
	// 5000 items per page this still admits a million entries before truncating.
	const maxListPages = 200
	items := make([]interface{}, 0)
	truncated := false
	pager := cc.NewListBlobsFlatPager(opts)
	for page := 0; pager.More(); page++ {
		if page >= maxListPages {
			truncated = true
			break
		}
		resp, err := pager.NextPage(flow.GoContext())
		if err != nil {
			if storage.HasCode(err, bloberror.ContainerNotFound) {
				return storage.ErrorResult(fmt.Sprintf("container %q was not found", containerName)), nil
			}
			_, msg := auth.SDKError(err)
			return storage.ErrorResult(msg), nil
		}
		if resp.Segment != nil {
			for _, item := range resp.Segment.BlobItems {
				items = append(items, blobItemMap(item))
			}
		}
		// Without return_all, a single page is the whole result.
		if !returnAll {
			break
		}
	}

	summary := fmt.Sprintf("Listed %d blobs in %s", len(items), containerName)
	if truncated {
		summary += " (stopped at the pagination safety cap; more remain)"
	}
	return storage.ListResult(items, summary), nil
}
