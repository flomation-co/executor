// Package oracle_filestorage_quota_rule_get reads one File Storage quota rule by
// its identifier within a file system.
package oracle_filestorage_quota_rule_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Get Quota Rule"
	Description  = "Fetch a single Oracle Cloud File Storage quota rule by its identifier within a file system — its display name, principal, hard/soft flag and gigabyte limit."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+gauge"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the file-system picker)"},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (scopes the file-system / export-set picker; not otherwise used)"},
	{Name: "file_system_ocid", Type: core.ConnectionTypeString, Label: "File System OCID", Placeholder: "ocid1.filesystem.oc1..aaaa…", Required: true},
	{Name: "quota_rule_ocid", Type: core.ConnectionTypeString, Label: "Quota Rule ID", Placeholder: "Base64 identifier of the quota rule", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "quota_rule", Type: core.ConnectionTypeObject, Label: "Quota Rule"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Quota Rule ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := fss.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	fileSystemID, err := fss.RequiredString("file_system_ocid", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	quotaRuleID, err := fss.RequiredString("quota_rule_ocid", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetQuotaRule(fss.Context(), filestorage.GetQuotaRuleRequest{
		FileSystemId: &fileSystemID,
		QuotaRuleId:  &quotaRuleID,
	})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}

	q := resp.QuotaRule
	rule := map[string]interface{}{
		"id":             fss.Str(q.Id),
		"display_name":   fss.Str(q.DisplayName),
		"file_system_id": fss.Str(q.FileSystemId),
		"principal_type": string(q.PrincipalType),
		"time_created":   fss.FormatTime(q.TimeCreated),
		"time_updated":   fss.FormatTime(q.TimeUpdated),
	}
	if q.IsHardQuota != nil {
		rule["is_hard_quota"] = *q.IsHardQuota
	}
	if q.QuotaLimitInGigabytes != nil {
		rule["quota_limit_in_gigabytes"] = *q.QuotaLimitInGigabytes
	}
	if q.PrincipalId != nil {
		rule["principal_id"] = *q.PrincipalId
	}

	quotaKind := "soft"
	if q.IsHardQuota != nil && *q.IsHardQuota {
		quotaKind = "hard"
	}
	limit := "unset"
	if q.QuotaLimitInGigabytes != nil {
		limit = fmt.Sprintf("%d GB", *q.QuotaLimitInGigabytes)
	}
	msg := fmt.Sprintf("Quota rule %q is a %s limit of %s", fss.Str(q.DisplayName), quotaKind, limit)

	return fss.Result(msg, map[string]interface{}{
		"quota_rule": rule,
		"id":         rule["id"],
	}), nil
}
