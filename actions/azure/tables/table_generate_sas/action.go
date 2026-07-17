package azure_tables_table_generate_sas

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	tables "flomation.app/automate/executor/actions/azure/tables"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Table Storage: Generate SAS Link"
	Description  = "Create a time-limited shared access signature for one table — hand out access without sharing the account key, optionally narrowed to a slice of rows. Signed locally with the account key; no API call is made"
	Website      = "https://www.flomation.co"
	Icon         = "azure+key"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

// defaultExpiryHours is the SAS lifetime when expiry_hours is unset.
const defaultExpiryHours = 24

var Inputs = [...]core.Connection{
	{Name: "account_name", Type: core.ConnectionTypeString, Label: "Storage Account", Placeholder: "mystorageaccount", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "shared_key", "entra"}}},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Shared Key", Value: "shared_key"}, {Name: "Connection String", Value: "connection_string"}, {Name: "Microsoft Entra (service principal)", Value: "entra"}}},
	{Name: "account_key", Type: core.ConnectionTypeSecret, Label: "Account Key", Placeholder: "Base64 account key — Storage Account ▸ Access keys", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "shared_key"}}},
	{Name: "connection_string", Type: core.ConnectionTypeSecret, Label: "Connection String", Placeholder: "DefaultEndpointsProtocol=https;AccountName=…;AccountKey=…;EndpointSuffix=core.windows.net — Storage Account ▸ Access keys ▸ Connection string", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"connection_string"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a Storage Table Data role on the account (RBAC)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://myaccount.table.core.windows.net — leave blank to derive; Azurite: http://host:10002/devstoreaccount1"},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip TLS verification — only for custom endpoints with a self-signed certificate"},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table", Placeholder: "MyTable — letters and digits only, starting with a letter", Required: true},
	{Name: "permissions", Type: core.ConnectionTypeString, Label: "Permissions", Placeholder: `r (read) is the default — any subset of "raud": read, add, update, delete`},
	{Name: "expiry_hours", Type: core.ConnectionTypeInteger, Label: "Expires After (hours)", Placeholder: "24 unless set"},
	{Name: "https_only", Type: core.ConnectionTypeBoolean, Label: "HTTPS Only", Placeholder: "Refuse the link over plain HTTP. Leave off for an Azurite endpoint, which is HTTP"},
	{Name: "start_partition_key", Type: core.ConnectionTypeString, Label: "From Partition Key", Placeholder: "Optional — limit the link to rows from this partition onwards"},
	{Name: "start_row_key", Type: core.ConnectionTypeString, Label: "From Row Key", Placeholder: "Optional — only meaningful alongside From Partition Key"},
	{Name: "end_partition_key", Type: core.ConnectionTypeString, Label: "To Partition Key", Placeholder: "Optional — limit the link to rows up to and including this partition"},
	{Name: "end_row_key", Type: core.ConnectionTypeString, Label: "To Row Key", Placeholder: "Optional — only meaningful alongside To Partition Key"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Table"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "SAS Details"},
	{Name: "sas_url", Type: core.ConnectionTypeString, Label: "SAS URL"},
	{Name: "sas_token", Type: core.ConnectionTypeString, Label: "SAS Token (query string)"},
	{Name: "expires_at", Type: core.ConnectionTypeString, Label: "Expires At"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := tables.GetAuth(inputs)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}
	table, err := tables.RequiredString("table", inputs)
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}
	if err := tables.ValidateTableName(table); err != nil {
		return tables.ErrorResult(err.Error()), nil
	}
	cred, err := auth.SharedKeyCredential()
	if err != nil {
		return tables.ErrorResult(err.Error()), nil
	}

	perms := tables.OptionalString("permissions", inputs)
	if perms == "" {
		perms = "r"
	}
	if err := (&aztables.SASPermissions{}).Parse(perms); err != nil {
		return tables.ErrorResult(fmt.Sprintf("permissions %q is not valid — use any subset of \"raud\" (read, add, update, delete)", perms)), nil
	}

	hours := defaultExpiryHours
	if h, set := tables.OptionalInt("expiry_hours", inputs); set {
		if h <= 0 {
			return tables.ErrorResult("expiry_hours must be greater than zero"), nil
		}
		hours = h
	}
	expiry := tables.Now().UTC().Add(time.Duration(hours) * time.Hour)

	values := aztables.SASSignatureValues{
		TableName:         table,
		Permissions:       perms,
		ExpiryTime:        expiry,
		StartPartitionKey: tables.OptionalString("start_partition_key", inputs),
		StartRowKey:       tables.OptionalString("start_row_key", inputs),
		EndPartitionKey:   tables.OptionalString("end_partition_key", inputs),
		EndRowKey:         tables.OptionalString("end_row_key", inputs),
	}
	if tables.OptionalBool("https_only", inputs) {
		values.Protocol = aztables.SASProtocolHTTPS
	}

	token, err := tables.SignTableSAS(values, cred)
	if err != nil {
		return tables.ErrorResult(auth.Errorf(err)), nil
	}

	// The table name goes into the URL exactly as the operator typed it, NOT
	// lowercased to match the signature's canonical name. The SDK lowercases
	// the name internally for signing and the service does the same when it
	// validates, so the signature still matches — but the name in the PATH is
	// what the service looks the table up by, and that lookup is
	// case-sensitive on Azurite. Lowercasing it here yields a link that passes
	// authentication and then 404s (verified against a live emulator).
	sasURL := auth.ServiceURL + "/" + table + "?" + token

	out := tables.ResourceResult(table, map[string]interface{}{
		"table":       table,
		"permissions": perms,
		"expires_at":  expiry.Format(time.RFC3339),
		"url":         sasURL,
	}, fmt.Sprintf("Signed a %s SAS for table %s, valid until %s", perms, table, expiry.Format(time.RFC3339)))
	out["sas_url"] = sasURL
	out["sas_token"] = token
	out["expires_at"] = expiry.Format(time.RFC3339)
	return out, nil
}
