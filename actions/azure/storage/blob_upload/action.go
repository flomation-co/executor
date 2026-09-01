package azure_storage_blob_upload

import (
	"bytes"
	"fmt"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Upload Blob"
	Description  = "Upload content to a block blob — inline text, base64, a flo:file reference, or an uploaded asset. Replaces the blob unless Overwrite is turned off. Uploads as a single Put Blob, so content is capped at the platform's 256 MB download/upload ceiling"
	Website      = "https://www.flomation.co"
	Icon         = "azure+arrow-up"
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
	{Name: "blob_name", Type: core.ConnectionTypeString, Placeholder: "reports/2026/summary.pdf",
		Label: "Blob Name (optional — a trailing / keeps the file's own name)"},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Content", Placeholder: "Text, base64, a flo:file reference, or ${parent.file} from an upstream node", Required: true},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type", Placeholder: "Detected from the content when blank (e.g. application/pdf)"},
	{
		Name:  "access_tier",
		Type:  core.ConnectionTypeString,
		Label: "Access Tier",
		Options: []core.ConnectionOption{
			{Name: "Account default", Value: ""},
			{Name: "Hot", Value: "Hot"},
			{Name: "Cool", Value: "Cool"},
			{Name: "Cold", Value: "Cold"},
			{Name: "Archive", Value: "Archive"},
		},
	},
	{Name: "metadata", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `{"source":"flomation"}`},
	{Name: "tags", Type: core.ConnectionTypeObject, Label: "Index Tags (JSON)", Placeholder: `{"project":"alpha"} — up to 10 searchable tags`},
	{Name: "overwrite", Type: core.ConnectionTypeBoolean, Label: "Overwrite", Placeholder: "On by default. Turn off to fail instead of replacing an existing blob", Value: true},
	{Name: "lease_id", Type: core.ConnectionTypeString, Label: "Lease ID", Placeholder: "Only needed when the blob or container is leased — the Lease ID output of a Lease step"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Blob Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Blob"},
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Blob URL"},
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
	blobName := storage.OptionalString("blob_name", inputs)
	contentConn := core.FindConnection("content", inputs)
	if contentConn == nil || contentConn.String() == nil || *contentConn.String() == "" {
		return storage.ErrorResult("content is required"), nil
	}

	// ResolveToBytes accepts every inbound media form: a flo:file: workspace
	// reference, a flo:blob: token, base64, or raw text.
	body, resolvedMime, err := flow.ResolveToBytes(*contentConn.String())
	if err != nil {
		return storage.ErrorResult(fmt.Sprintf("failed to resolve content: %s", err.Error())), nil
	}

	// A blank name, or one ending in "/", means "put it in there under its own
	// name" rather than creating a blob literally called "" or "prefix/".
	blobName = core.UploadDestination(blobName, *contentConn.String(), resolvedMime, "upload")

	contentType := storage.OptionalString("content_type", inputs)
	if contentType == "" {
		contentType = resolvedMime
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	meta, err := storage.MetadataMap(inputs, "metadata")
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	tags, err := storage.StringMapInput("tags", inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	if len(tags) > 0 {
		if err := storage.ValidateTags(tags); err != nil {
			return storage.ErrorResult(err.Error()), nil
		}
	}

	// This action writes block blobs only. Page and append blobs need
	// alignment/append-block protocols a single Put Blob cannot express — n8n
	// offers the dropdown, but only BlockBlob actually works there either.
	bb, err := auth.BlockBlobClient(container, blobName)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	opts := &blockblob.UploadOptions{
		HTTPHeaders: &blob.HTTPHeaders{BlobContentType: &contentType},
		Metadata:    meta,
	}
	if len(tags) > 0 {
		opts.Tags = tags
	}
	if tier := storage.OptionalString("access_tier", inputs); tier != "" {
		at := blob.AccessTier(tier)
		opts.Tier = &at
	}
	ac := &blob.AccessConditions{}
	if lid := storage.LeaseIDPtr(inputs); lid != nil {
		ac.LeaseAccessConditions = &blob.LeaseAccessConditions{LeaseID: lid}
	}
	if !storage.BoolDefaultTrue("overwrite", inputs) {
		// If-None-Match: * makes the create conditional on absence — the
		// service answers 409 BlobAlreadyExists instead of replacing.
		ac.ModifiedAccessConditions = &blob.ModifiedAccessConditions{IfNoneMatch: to.Ptr(azcore.ETagAny)}
	}
	if ac.LeaseAccessConditions != nil || ac.ModifiedAccessConditions != nil {
		opts.AccessConditions = ac
	}

	// Single Put Blob (blockblob.Upload), preserving this action's contract —
	// the platform caps content at its download/upload ceiling upstream.
	resp, err := bb.Upload(flow.GoContext(), streaming.NopCloser(bytes.NewReader(body)), opts)
	if err != nil {
		if storage.HasCode(err, bloberror.BlobAlreadyExists) || storage.HasCode(err, bloberror.ConditionNotMet) {
			return storage.ErrorResult(fmt.Sprintf("blob %q already exists in %q and Overwrite is off", blobName, container)), nil
		}
		_, msg := auth.SDKError(err)
		return storage.ErrorResult(msg), nil
	}

	out := storage.WriteResult(blobName, resp.ETag, resp.LastModified,
		fmt.Sprintf("Uploaded %s to %s (%d bytes)", blobName, container, len(body)))
	out["url"] = auth.BaseURL + storage.BlobPath(container, blobName)
	return out, nil
}
