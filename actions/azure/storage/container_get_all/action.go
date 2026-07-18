package azure_storage_container_get_all

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: List Containers"
	Description  = "List the containers in the storage account, optionally filtered by a name prefix and enriched with metadata, soft-deleted containers, or system containers"
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
	{Name: "prefix", Type: core.ConnectionTypeString, Label: "Prefix", Placeholder: "Only containers whose name starts with this"},
	{
		Name:        "include",
		Type:        core.ConnectionTypeComboBox,
		Label:       "Include",
		Placeholder: "Leave blank for nothing extra; combine with commas, e.g. metadata,deleted",
		Options: []core.ConnectionOption{
			{Name: "Metadata", Value: "metadata"},
			{Name: "Soft-deleted containers", Value: "deleted"},
			{Name: "Metadata + soft-deleted containers", Value: "metadata,deleted"},
			{Name: "System containers ($logs, $blobchangefeed)", Value: "system"},
		},
	},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow pagination until every container is fetched"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max containers to return when not returning all (default 50, max 5000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Containers"},
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

	// maxListPages bounds a return_all page walk so an account with a very large
	// number of containers can never spin unbounded requests — the same backstop
	// the pre-SDK ListEnumeration applied. At 5000 per page this admits a million.
	const maxListPages = 200

	// containerItemMap reproduces the pre-SDK storage.ContainerMap output shape
	// from the SDK's typed service.ContainerItem: the same {name, properties,
	// metadata, deleted} keys, properties camelCased exactly as the XML path did.
	containerItemMap := func(item *service.ContainerItem) map[string]interface{} {
		out := map[string]interface{}{}
		if item.Name != nil {
			out["name"] = *item.Name
		}
		if p := item.Properties; p != nil {
			props := map[string]interface{}{}
			if p.ETag != nil {
				props["etag"] = string(*p.ETag)
			}
			if p.LastModified != nil {
				props["lastModified"] = p.LastModified.UTC().Format(time.RFC1123)
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
			if p.PublicAccess != nil {
				props["publicAccess"] = string(*p.PublicAccess)
			}
			if p.HasImmutabilityPolicy != nil {
				props["hasImmutabilityPolicy"] = *p.HasImmutabilityPolicy
			}
			if p.HasLegalHold != nil {
				props["hasLegalHold"] = *p.HasLegalHold
			}
			if len(props) > 0 {
				out["properties"] = props
			}
		}
		if meta := storage.StrMeta(item.Metadata); len(meta) > 0 {
			out["metadata"] = meta
		}
		if item.Deleted != nil {
			out["deleted"] = *item.Deleted
		}
		return out
	}

	svc, err := auth.ServiceClient()
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	opts := &service.ListContainersOptions{}
	if prefix := storage.OptionalString("prefix", inputs); prefix != "" {
		opts.Prefix = &prefix
	}
	// ParseIncludeTokens still validates the ComboBox input (unknown token → the
	// same error), then the accepted tokens map onto the SDK's include booleans.
	include, err := storage.ParseIncludeTokens(storage.OptionalString("include", inputs), storage.ContainerIncludeTokens)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	for _, tok := range strings.Split(include, ",") {
		switch tok {
		case "metadata":
			opts.Include.Metadata = true
		case "deleted":
			opts.Include.Deleted = true
		case "system":
			opts.Include.System = true
		}
	}
	returnAll := storage.OptionalBool("return_all", inputs)
	limit := storage.ClampLimit(storage.OptionalInt("limit", inputs))
	pageSize := limit
	if returnAll {
		// return_all raises the page size to the 5000 maximum to minimise round
		// trips, exactly as ListEnumeration did.
		pageSize = storage.MaxPageLimit
	}
	mr := int32(pageSize)
	opts.MaxResults = &mr

	pager := svc.NewListContainersPager(opts)
	items := make([]interface{}, 0)
	truncated := false
	ctx := flow.GoContext()
	for page := 0; pager.More(); page++ {
		if page >= maxListPages {
			truncated = true
			break
		}
		resp, err := pager.NextPage(ctx)
		if err != nil {
			_, msg := auth.SDKError(err)
			return storage.ErrorResult(msg), nil
		}
		for _, item := range resp.ContainerItems {
			items = append(items, containerItemMap(item))
		}
		// Without return_all this is a single page (limit items); with it, walk
		// the cursor until exhausted or the page cap trips.
		if !returnAll {
			break
		}
	}

	summary := fmt.Sprintf("Listed %d containers", len(items))
	if truncated {
		summary += " (stopped at the pagination safety cap; more remain)"
	}
	return storage.ListResult(items, summary), nil
}
