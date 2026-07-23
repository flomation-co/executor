// Package oracle_filestorage_quota_rule_list lists the quota rules (and current usage)
// on one NFS file system. Quota rules are scoped by FILE SYSTEM (not compartment or
// availability domain), and the File Storage API requires a principal type on every
// listing — so both the file system OCID and the principal type are required here.
package oracle_filestorage_quota_rule_list

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
	Name         = "OCI File Storage: List Quota Rules"
	Description  = "List the quota rules and current usage on one Oracle Cloud NFS file system, for a given principal type (FILE_SYSTEM_LEVEL, DEFAULT_USER/GROUP or INDIVIDUAL_USER/GROUP), optionally limited to principals that violate their quota. Walks pagination up to a safe cap."
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
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (scopes the file-system picker)"},
	{Name: "file_system_ocid", Type: core.ConnectionTypeString, Label: "File System OCID", Placeholder: "ocid1.filesystem.oc1..aaaa…", Required: true},
	{Name: "principal_type", Type: core.ConnectionTypeString, Label: "Principal Type", Placeholder: "FILE_SYSTEM_LEVEL | DEFAULT_USER | DEFAULT_GROUP | INDIVIDUAL_USER | INDIVIDUAL_GROUP", Required: true},
	{Name: "are_violators_only", Type: core.ConnectionTypeBoolean, Label: "Violators Only", Placeholder: "Only report principals that exceed their quota (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "quota_rules", Type: core.ConnectionTypeObject, Label: "Quota Rules"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, fileSystemID, errResult := fss.ResourceClient(inputs, "file_system_ocid")
	if errResult != nil {
		return errResult, nil
	}
	rawType, err := fss.RequiredString("principal_type", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	principalType, ok := filestorage.GetMappingListQuotaRulesPrincipalTypeEnum(rawType)
	if !ok {
		return fss.ErrorResult(fmt.Sprintf("principal type %q is not valid — expected one of: %s", rawType, strings.Join(filestorage.GetListQuotaRulesPrincipalTypeEnumStringValues(), ", "))), nil
	}

	req := filestorage.ListQuotaRulesRequest{FileSystemId: &fileSystemID, PrincipalType: principalType}
	if fss.BoolWasSet("are_violators_only", inputs) {
		v := fss.OptionalBool("are_violators_only", inputs, false)
		req.AreViolatorsOnly = &v
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= fss.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListQuotaRules(fss.Context(), req)
		if err != nil {
			return fss.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, summariseQuotaRuleSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return fss.Result(fmt.Sprintf("Found %d quota rule(s)", len(out)), map[string]interface{}{
		"quota_rules": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}

// summariseQuotaRuleSummary flattens a QuotaRuleSummary — the long-tail type has no shared
// summariser in common.go, so it is built inline from the SDK struct.
func summariseQuotaRuleSummary(q *filestorage.QuotaRuleSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                       fss.Str(q.Id),
		"display_name":             fss.Str(q.DisplayName),
		"file_system_id":           fss.Str(q.FileSystemId),
		"principal_type":           string(q.PrincipalType),
		"principal_id":             intOrNil(q.PrincipalId),
		"is_hard_quota":            boolOrNil(q.IsHardQuota),
		"quota_limit_in_gigabytes": intOrNil(q.QuotaLimitInGigabytes),
		"usage_in_bytes":           fss.Int64OrNil(q.UsageInBytes),
		"are_violators_only":       boolOrNil(q.AreViolatorsOnly),
		"time_created":             fss.FormatTime(q.TimeCreated),
		"time_updated":             fss.FormatTime(q.TimeUpdated),
	}
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
