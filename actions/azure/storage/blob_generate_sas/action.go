package azure_storage_blob_generate_sas

import (
	"fmt"
	"net"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Generate SAS Link"
	Description  = "Create a time-limited shared access signature URL for a blob or container — hand out downloads or accept uploads without sharing the account key. Signed locally with the account key; no API call is made"
	Website      = "https://www.flomation.co"
	Icon         = "azure+key"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

// defaultExpiryHours is the SAS lifetime when neither expiry input is set.
const defaultExpiryHours = 24

var Inputs = [...]core.Connection{
	{Name: "account_name", Type: core.ConnectionTypeString, Label: "Storage Account", Placeholder: "mystorageaccount", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Shared Key", Value: "shared_key"}, {Name: "Microsoft Entra (service principal)", Value: "entra"}}},
	{Name: "account_key", Type: core.ConnectionTypeSecret, Label: "Account Key", Placeholder: "Base64 account key — Storage Account ▸ Access keys", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "shared_key"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a Storage Blob Data role on the account (RBAC)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://myaccount.blob.core.windows.net — leave blank to derive; Azurite: http://host:10000/devstoreaccount1"},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip TLS verification — only for custom endpoints with a self-signed certificate"},
	{
		Name:  "resource",
		Type:  core.ConnectionTypeString,
		Label: "Resource",
		Options: []core.ConnectionOption{
			{Name: "Blob", Value: "blob"},
			{Name: "Container", Value: "container"},
		},
	},
	{Name: "container", Type: core.ConnectionTypeString, Label: "Container", Placeholder: "my-container", Required: true},
	{Name: "blob_name", Type: core.ConnectionTypeString, Label: "Blob Name", Placeholder: "reports/2026/summary.pdf", Visible: &core.VisibleWhen{Field: "resource", Values: []string{"", "blob"}}},
	{Name: "permissions", Type: core.ConnectionTypeString, Label: "Permissions", Placeholder: `r (read) is the default — subset of "racwdxltmei", in that order`},
	{Name: "expiry_hours", Type: core.ConnectionTypeInteger, Label: "Expires After (hours)", Placeholder: "24 unless set — ignored when an explicit expiry is given"},
	{Name: "expiry", Type: core.ConnectionTypeDateTime, Label: "Expiry", Placeholder: "Explicit expiry timestamp — wins over Expires After"},
	{Name: "start", Type: core.ConnectionTypeDateTime, Label: "Start", Placeholder: "Optional not-before timestamp (defaults to immediately valid)"},
	{Name: "ip_range", Type: core.ConnectionTypeString, Label: "Allowed IP Range", Placeholder: "168.1.5.65 or 168.1.5.60-168.1.5.70"},
	{Name: "content_disposition", Type: core.ConnectionTypeString, Label: "Content Disposition", Placeholder: `attachment; filename="report.pdf" — forced on downloads via this link`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Resource Path"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "SAS Details"},
	{Name: "sas_url", Type: core.ConnectionTypeString, Label: "SAS URL"},
	{Name: "sas_token", Type: core.ConnectionTypeString, Label: "SAS Token (query string)"},
	{Name: "expires_at", Type: core.ConnectionTypeString, Label: "Expires At"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// parseTimeInput accepts the formats a DateTime input plausibly carries.
func parseTimeInput(name, v string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%s %q is not a recognised timestamp (use RFC3339, e.g. 2026-07-17T10:00:00Z)", name, v)
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := storage.GetAuth(inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	// A service SAS is signed with the ACCOUNT KEY; an Entra service principal
	// has no key to sign with (user-delegation SAS is a different flow).
	if auth.Method != storage.AuthSharedKey {
		return storage.ErrorResult("SAS generation requires Shared Key auth"), nil
	}
	container, err := storage.RequiredString("container", inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	resource := storage.OptionalString("resource", inputs)
	if resource == "" {
		resource = "blob"
	}
	params := storage.SASParams{
		Container:          container,
		IP:                 storage.OptionalString("ip_range", inputs),
		ContentDisposition: storage.OptionalString("content_disposition", inputs),
	}
	var resourcePath string
	switch resource {
	case "blob":
		params.Resource = "b"
		blobName, err := storage.RequiredString("blob_name", inputs)
		if err != nil {
			return storage.ErrorResult(err.Error()), nil
		}
		params.Blob = blobName
		resourcePath = storage.BlobPath(container, blobName)
	case "container":
		params.Resource = "c"
		resourcePath = storage.ContainerPath(container)
	default:
		return storage.ErrorResult(fmt.Sprintf("resource %q is not valid: use blob or container", resource)), nil
	}

	params.Permissions = storage.OptionalString("permissions", inputs)
	if params.Permissions == "" {
		params.Permissions = "r"
	}
	if err := storage.ValidateSASPermissions(params.Permissions); err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	// Expiry: an explicit timestamp wins; otherwise now + expiry_hours
	// (default 24).
	if expiryStr := storage.OptionalString("expiry", inputs); expiryStr != "" {
		t, err := parseTimeInput("expiry", expiryStr)
		if err != nil {
			return storage.ErrorResult(err.Error()), nil
		}
		params.Expiry = t
	} else {
		hours, set := storage.OptionalInt("expiry_hours", inputs)
		if !set || hours <= 0 {
			hours = defaultExpiryHours
		}
		params.Expiry = time.Now().Add(time.Duration(hours) * time.Hour)
	}
	if startStr := storage.OptionalString("start", inputs); startStr != "" {
		t, err := parseTimeInput("start", startStr)
		if err != nil {
			return storage.ErrorResult(err.Error()), nil
		}
		params.Start = t
	}
	if !params.Start.IsZero() && !params.Expiry.After(params.Start) {
		return storage.ErrorResult("expiry must be after start"), nil
	}
	if params.Expiry.Before(time.Now()) {
		return storage.ErrorResult("expiry is in the past"), nil
	}

	// The service SAS is signed by the SDK now (sas.BlobSignatureValues), not
	// by a hand-rolled string-to-sign. This is the whole point of the migration
	// as it applies to signing: the Blob SAS layout is exactly the kind of
	// slot-order detail that is better owned by code Microsoft maintains — and
	// the sibling File SAS (azure/files, still REST) proved in wave 2 how a
	// one-slot mistake ships a 100%-broken link that only a live fetch catches.
	//
	// The permission string is still validated in canonical order above (so an
	// operator learns the rule rather than getting a silently reordered token),
	// then handed straight to the SDK. No Protocol (spr) is set — matching the
	// pre-SDK token, so the link works over http against Azurite as well as
	// https against real Azure.
	cred, err := auth.SharedKeyCredential()
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	// .UTC() is load-bearing. The SDK's sas signer formats st/se with a literal
	// "Z" suffix taken from the value's OWN wall-clock — it does NOT convert to
	// UTC first. So a local-zoned expiry (the default path is time.Now().Add(),
	// which is server-local) would be stamped e.g. "11:00:00Z" while the real
	// instant is 10:00 UTC, and the token would outlive the expires_at output
	// (computed via .UTC()) by the server's UTC offset. Harmless on a UTC
	// server; wrong by an hour on a BST one. Convert here so the signed token
	// and the reported expiry always agree, on any server timezone.
	vals := sas.BlobSignatureValues{
		StartTime:          params.Start.UTC(), // zero value stays zero → omitted (st)
		ExpiryTime:         params.Expiry.UTC(),
		Permissions:        params.Permissions,
		ContainerName:      container,
		BlobName:           params.Blob, // "" for a container SAS
		ContentDisposition: params.ContentDisposition,
	}
	if params.IP != "" {
		ipRange, err := parseSASIPRange(params.IP)
		if err != nil {
			return storage.ErrorResult(err.Error()), nil
		}
		vals.IPRange = ipRange
	}
	qp, err := vals.SignWithSharedKey(cred)
	if err != nil {
		_, msg := auth.SDKError(err)
		return storage.ErrorResult(fmt.Sprintf("failed to sign the SAS: %s", msg)), nil
	}
	token := qp.Encode()

	expiresAt := params.Expiry.UTC().Format(time.RFC3339)
	sasURL := auth.BaseURL + resourcePath + "?" + token
	result := map[string]interface{}{
		"resource":    resource,
		"permissions": params.Permissions,
		"expiresAt":   expiresAt,
	}
	if !params.Start.IsZero() {
		result["startsAt"] = params.Start.UTC().Format(time.RFC3339)
	}

	out := storage.ResourceResult(strings.TrimPrefix(resourcePath, "/"), result,
		fmt.Sprintf("Generated a %q SAS link for %s expiring %s", params.Permissions, strings.TrimPrefix(resourcePath, "/"), expiresAt))
	out["sas_url"] = sasURL
	out["sas_token"] = token
	out["expires_at"] = expiresAt
	return out, nil
}

// parseSASIPRange turns the ip_range input ("1.2.3.4" or "1.2.3.4-5.6.7.8")
// into the SDK's IPRange. The pre-SDK token placed the raw string into sip
// verbatim; the SDK formats it from net.IP, so an invalid value now fails by
// rule here rather than producing a token the service silently rejects.
//
// net.ParseIP also accepts IPv6, and the SDK formats it into sip without error
// (verified) — but Azure's SAS signed-IP restriction is IPv4-only, so an IPv6
// range yields a token the service refuses at fetch time. That is unchanged
// from the pre-SDK pass-through, so it is left as-is rather than rejected here.
func parseSASIPRange(raw string) (sas.IPRange, error) {
	var r sas.IPRange
	parts := strings.SplitN(raw, "-", 2)
	start := net.ParseIP(strings.TrimSpace(parts[0]))
	if start == nil {
		return r, fmt.Errorf("ip_range start %q is not a valid IP address", strings.TrimSpace(parts[0]))
	}
	r.Start = start
	if len(parts) == 2 {
		end := net.ParseIP(strings.TrimSpace(parts[1]))
		if end == nil {
			return r, fmt.Errorf("ip_range end %q is not a valid IP address", strings.TrimSpace(parts[1]))
		}
		r.End = end
	}
	return r, nil
}
