// Package oracle_filestorage_quota_rule_update edits a file-system quota rule's
// display name and/or limit.
package oracle_filestorage_quota_rule_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Update Quota Rule"
	Description  = "Update an Oracle Cloud file-system quota rule's display name and/or gigabyte limit. Only the fields you supply are changed; the rest are left as-is. (Whether a rule is a hard quota is fixed at creation and cannot be edited.)"
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
	{Name: "quota_rule_ocid", Type: core.ConnectionTypeString, Label: "Quota Rule Identifier", Placeholder: "The quota rule identifier (base64 principal tuple)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep the current one)"},
	{Name: "quota_limit_gb", Type: core.ConnectionTypeInteger, Label: "Quota Limit (GB)", Placeholder: "New limit in gigabytes (leave blank to keep)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "quota_rule", Type: core.ConnectionTypeObject, Label: "Quota Rule"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Quota Rule Identifier"},
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

	var details filestorage.UpdateQuotaRuleDetails
	changed := false
	if name := strings.TrimSpace(fss.OptionalString("display_name", inputs)); name != "" {
		details.DisplayName = &name
		changed = true
	}
	if n, ok, err := fss.OptionalInt("quota_limit_gb", inputs); err != nil {
		return fss.ErrorResult(err.Error()), nil
	} else if ok {
		limit := n
		details.QuotaLimitInGigabytes = &limit
		changed = true
	}
	if !changed {
		return fss.ErrorResult("nothing to update — supply a display name and/or quota limit (GB)"), nil
	}

	resp, err := client.UpdateQuotaRule(fss.Context(), filestorage.UpdateQuotaRuleRequest{
		FileSystemId:           &fileSystemID,
		QuotaRuleId:            &quotaRuleID,
		UpdateQuotaRuleDetails: details,
	})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}

	rule := resp.QuotaRule
	summary := map[string]interface{}{
		"id":                    fss.Str(rule.Id),
		"file_system_id":        fss.Str(rule.FileSystemId),
		"display_name":          fss.Str(rule.DisplayName),
		"is_hard_quota":         boolOrNil(rule.IsHardQuota),
		"quota_limit_gigabytes": intOrNil(rule.QuotaLimitInGigabytes),
		"principal_type":        string(rule.PrincipalType),
		"principal_id":          intOrNil(rule.PrincipalId),
		"time_created":          fss.FormatTime(rule.TimeCreated),
		"time_updated":          fss.FormatTime(rule.TimeUpdated),
	}
	return fss.Result(fmt.Sprintf("Updated quota rule %q on file system %s", summary["display_name"], fileSystemID), map[string]interface{}{
		"quota_rule": summary,
		"id":         summary["id"],
	}), nil
}

func intOrNil(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func boolOrNil(p *bool) interface{} {
	if p == nil {
		return nil
	}
	return *p
}
