package azure_files_file_generate_sas

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	files "flomation.app/automate/executor/actions/azure/files"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Files: Generate SAS Link"
	Description  = "Create a time-limited shared access signature URL for a file or a whole share — hand out downloads or accept uploads without sharing the account key. Signed locally with the account key; no API call is made"
	Website      = "https://www.flomation.co"
	Icon         = "azure+key"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "account_name", Type: core.ConnectionTypeString, Label: "Storage Account", Placeholder: "mystorageaccount", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Shared Key", Value: "shared_key"}, {Name: "Microsoft Entra (service principal)", Value: "entra"}}},
	{Name: "account_key", Type: core.ConnectionTypeSecret, Label: "Account Key", Placeholder: "Base64 account key — Storage Account ▸ Access keys", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "shared_key"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a Storage File Data SMB/Privileged role. Azure requires backup intent on OAuth calls, which BYPASSES the share's file permissions — use Shared Key if the ACLs must apply", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://myaccount.file.core.windows.net — leave blank to derive; sovereign clouds only (Azurite has no File service)"},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip TLS verification — only for custom endpoints with a self-signed certificate"},
	{
		Name:  "resource",
		Type:  core.ConnectionTypeString,
		Label: "Resource",
		Options: []core.ConnectionOption{
			{Name: "File", Value: "file"},
			{Name: "Share", Value: "share"},
		},
	},
	{Name: "share", Type: core.ConnectionTypeString, Label: "Share", Placeholder: "my-share", Required: true},
	{Name: "path", Type: core.ConnectionTypeString, Label: "File Path", Placeholder: "reports/2026/summary.pdf", Visible: &core.VisibleWhen{Field: "resource", Values: []string{"", "file"}}},
	{Name: "permissions", Type: core.ConnectionTypeString, Label: "Permissions", Placeholder: `r (read) is the default — subset of "rcwdl" (read, create, write, delete, list), in that order; list applies to a share only`},
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

// defaultExpiryHours is the SAS lifetime when neither expiry input is set.
const defaultExpiryHours = 24

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
	return time.Time{}, fmt.Errorf("%s %q is not a recognised timestamp (use RFC3339, e.g. 2026-07-18T10:00:00Z)", name, v)
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := files.GetAuth(inputs)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	// A service SAS is signed with the ACCOUNT KEY; an Entra service principal
	// has no key to sign with (user-delegation SAS is a different flow, and on
	// Files it would carry the backup-intent ACL bypass besides).
	if auth.Method != files.AuthSharedKey {
		return files.ErrorResult("SAS generation requires Shared Key auth"), nil
	}
	share, err := files.RequiredString("share", inputs)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}

	resource := files.OptionalString("resource", inputs)
	if resource == "" {
		resource = "file"
	}
	params := files.SASParams{
		Share:              share,
		IP:                 files.OptionalString("ip_range", inputs),
		ContentDisposition: files.OptionalString("content_disposition", inputs),
	}
	var resourcePath string
	switch resource {
	case "file":
		params.Resource = files.SASResourceFile
		p, err := files.RequiredString("path", inputs)
		if err != nil {
			return files.ErrorResult(err.Error()), nil
		}
		p = strings.Trim(p, "/")
		if err := files.ValidateFilePath("path", p); err != nil {
			return files.ErrorResult(err.Error()), nil
		}
		params.Path = p
		resourcePath = files.DirectoryPath(share, p)
	case "share":
		params.Resource = files.SASResourceShare
		resourcePath = files.SharePath(share)
	default:
		return files.ErrorResult(fmt.Sprintf("resource %q is not valid: use file or share", resource)), nil
	}

	params.Permissions = files.OptionalString("permissions", inputs)
	if params.Permissions == "" {
		params.Permissions = "r"
	}
	if err := files.ValidateSASPermissions(params.Permissions, params.Resource); err != nil {
		return files.ErrorResult(err.Error()), nil
	}

	// Expiry: an explicit timestamp wins; otherwise now + expiry_hours
	// (default 24).
	if expiryStr := files.OptionalString("expiry", inputs); expiryStr != "" {
		t, err := parseTimeInput("expiry", expiryStr)
		if err != nil {
			return files.ErrorResult(err.Error()), nil
		}
		params.Expiry = t
	} else {
		hours, set := files.OptionalInt("expiry_hours", inputs)
		if !set || hours <= 0 {
			hours = defaultExpiryHours
		}
		params.Expiry = time.Now().Add(time.Duration(hours) * time.Hour)
	}
	if startStr := files.OptionalString("start", inputs); startStr != "" {
		t, err := parseTimeInput("start", startStr)
		if err != nil {
			return files.ErrorResult(err.Error()), nil
		}
		params.Start = t
	}
	if !params.Start.IsZero() && !params.Expiry.After(params.Start) {
		return files.ErrorResult("expiry must be after start"), nil
	}
	if params.Expiry.Before(time.Now()) {
		return files.ErrorResult("expiry is in the past"), nil
	}

	token, err := files.BuildServiceSAS(auth, params)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}

	expiresAt := params.Expiry.UTC().Format(time.RFC3339)
	sasURL := auth.BaseURL + resourcePath + "?" + token
	id := strings.TrimPrefix(resourcePath, "/")
	result := map[string]interface{}{
		"resource":    resource,
		"permissions": params.Permissions,
		"expiresAt":   expiresAt,
	}
	if !params.Start.IsZero() {
		result["startsAt"] = params.Start.UTC().Format(time.RFC3339)
	}

	out := files.ResourceResult(id, result,
		fmt.Sprintf("Generated a %q SAS link for %s expiring %s", params.Permissions, id, expiresAt))
	out["sas_url"] = sasURL
	out["sas_token"] = token
	out["expires_at"] = expiresAt
	return out, nil
}
