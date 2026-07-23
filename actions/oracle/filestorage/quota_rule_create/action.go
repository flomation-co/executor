// Package oracle_filestorage_quota_rule_create adds a quota rule to an Oracle
// Cloud (OCI) NFS file system — capping the logical space a user, a group, or
// the whole file system may consume.
package oracle_filestorage_quota_rule_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Create Quota Rule"
	Description  = "Add a quota rule to an Oracle Cloud NFS file system, limiting the logical space a user, a group, or the whole file system may consume."
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
	{Name: "file_system_ocid", Type: core.ConnectionTypeString, Label: "File System OCID", Placeholder: "ocid1.filesystem.oc1..aaaa… — the file system the quota rule belongs to", Required: true},
	{Name: "principal_type", Type: core.ConnectionTypeString, Label: "Principal Type", Placeholder: "FILE_SYSTEM_LEVEL, DEFAULT_USER, DEFAULT_GROUP, INDIVIDUAL_USER or INDIVIDUAL_GROUP", Required: true},
	{Name: "principal_id", Type: core.ConnectionTypeString, Label: "Principal ID (UID/GID)", Placeholder: "UNIX user/group numeric id — required for INDIVIDUAL_USER / INDIVIDUAL_GROUP"},
	{Name: "quota_limit_gb", Type: core.ConnectionTypeString, Label: "Quota Limit (GB)", Placeholder: "e.g. 100 — the limit in gigabytes", Required: true},
	{Name: "is_hard_quota", Type: core.ConnectionTypeBoolean, Label: "Hard Quota", Placeholder: "On = block writes past the limit; Off = soft quota (writes succeed but the rule is violated)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A friendly label, e.g. \"UserXYZ's quota\" (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "quota_rule", Type: core.ConnectionTypeObject, Label: "Quota Rule"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Quota Rule ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, fileSystemID, errResult := fss.ResourceClient(inputs, "file_system_ocid")
	if errResult != nil {
		return errResult, nil
	}

	principalRaw, err := fss.RequiredString("principal_type", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	principalType, ok := filestorage.GetMappingCreateQuotaRuleDetailsPrincipalTypeEnum(principalRaw)
	if !ok {
		return fss.ErrorResult(fmt.Sprintf("principal type %q is not valid — expected one of FILE_SYSTEM_LEVEL, DEFAULT_USER, DEFAULT_GROUP, INDIVIDUAL_USER, INDIVIDUAL_GROUP", principalRaw)), nil
	}

	limit, limitSet, err := fss.OptionalInt("quota_limit_gb", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	if !limitSet {
		return fss.ErrorResult("quota limit (GB) is required"), nil
	}

	hard := fss.OptionalBool("is_hard_quota", inputs, false)

	details := filestorage.CreateQuotaRuleDetails{
		PrincipalType:         principalType,
		IsHardQuota:           &hard,
		QuotaLimitInGigabytes: &limit,
	}
	if principalID, idSet, err := fss.OptionalInt("principal_id", inputs); err != nil {
		return fss.ErrorResult(err.Error()), nil
	} else if idSet {
		details.PrincipalId = &principalID
	}
	if name := fss.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}

	resp, err := client.CreateQuotaRule(fss.Context(), filestorage.CreateQuotaRuleRequest{
		FileSystemId:           &fileSystemID,
		CreateQuotaRuleDetails: details,
	})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}

	qr := resp.QuotaRule
	rule := map[string]interface{}{
		"id":             fss.Str(qr.Id),
		"display_name":   fss.Str(qr.DisplayName),
		"file_system_id": fss.Str(qr.FileSystemId),
		"principal_type": string(qr.PrincipalType),
		"principal_id":   intOrNil(qr.PrincipalId),
		"quota_limit":    intOrNil(qr.QuotaLimitInGigabytes),
		"is_hard_quota":  boolOrNil(qr.IsHardQuota),
		"time_created":   fss.FormatTime(qr.TimeCreated),
		"time_updated":   fss.FormatTime(qr.TimeUpdated),
	}

	kind := "soft"
	if hard {
		kind = "hard"
	}
	return fss.Result(fmt.Sprintf("Created %s quota rule of %d GB for %s on file system %s", kind, limit, principalType, fileSystemID), map[string]interface{}{
		"quota_rule": rule,
		"id":         rule["id"],
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
