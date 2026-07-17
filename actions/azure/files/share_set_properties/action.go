package azure_files_share_set_properties

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	files "flomation.app/automate/executor/actions/azure/files"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Files: Set Share Properties"
	Description  = "Change a share's size quota or access tier. Both are optional, but at least one must be given — an empty call would silently do nothing"
	Website      = "https://www.flomation.co"
	Icon         = "azure+gear"
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
	{Name: "share", Type: core.ConnectionTypeString, Label: "Share", Placeholder: "my-share", Required: true},
	{Name: "quota", Type: core.ConnectionTypeInteger, Label: "Quota (GiB)", Placeholder: "New maximum size, 1-102400 — leave blank to keep the current quota"},
	{
		Name:        "access_tier",
		Type:        core.ConnectionTypeString,
		Label:       "Access Tier",
		Placeholder: "Leave blank to keep the current tier",
		Options: []core.ConnectionOption{
			{Name: "Keep the current tier", Value: ""},
			{Name: "Transaction Optimized", Value: "TransactionOptimized"},
			{Name: "Hot", Value: "Hot"},
			{Name: "Cool", Value: "Cool"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Share"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Share"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

const (
	minQuotaGiB = 1
	maxQuotaGiB = 102400
)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := files.GetAuth(inputs)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	share, err := files.RequiredString("share", inputs)
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}

	headers := map[string]string{}
	changed := []string{}
	if quota, set := files.OptionalInt("quota", inputs); set {
		if quota < minQuotaGiB || quota > maxQuotaGiB {
			return files.ErrorResult(fmt.Sprintf("quota must be between %d and %d GiB (got %d)", minQuotaGiB, maxQuotaGiB, quota)), nil
		}
		headers["x-ms-share-quota"] = strconv.Itoa(quota)
		changed = append(changed, fmt.Sprintf("quota %d GiB", quota))
	}
	if tier := files.OptionalString("access_tier", inputs); tier != "" {
		headers["x-ms-access-tier"] = tier
		changed = append(changed, "tier "+tier)
	}
	// An omitted header is "leave it alone", so a call with neither is a no-op
	// the service happily reports as a success. Say so instead.
	if len(changed) == 0 {
		return files.ErrorResult("set at least one of quota or access_tier — with both blank this step would change nothing"), nil
	}

	resp, err := files.Do(flow, auth, files.Request{
		Method:  http.MethodPut,
		Path:    files.SharePath(share),
		Query:   url.Values{"restype": []string{"share"}, "comp": []string{"properties"}},
		Headers: headers,
	})
	if err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	if err := files.CheckResponse(resp); err != nil {
		return files.ErrorResult(err.Error()), nil
	}
	return files.ResourceResult(share, files.HeadersResult(share, resp.Headers),
		fmt.Sprintf("Set %s on share %s", strings.Join(changed, " and "), share)), nil
}
