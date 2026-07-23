// Package oracle_autonomousdatabase_db_list_character_sets lists the valid Oracle
// character sets available when provisioning an Autonomous Database — either the
// database character sets or the national character sets.
package oracle_autonomousdatabase_db_list_character_sets

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	adb "flomation.app/automate/executor/actions/oracle/autonomousdatabase"

	"github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Autonomous Database: List Character Sets"
	Description  = "List the valid database or national character sets for provisioning an Oracle Cloud Autonomous Database."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
	Date         = "21/07/2026"
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
	{Name: "character_set_type", Type: core.ConnectionTypeString, Label: "Character Set Type", Placeholder: "Database or national character sets (optional)", Options: []core.ConnectionOption{
		{Name: "Database Character Sets", Value: "DATABASE"},
		{Name: "National Character Sets", Value: "NATIONAL"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "character_sets", Type: core.ConnectionTypeObject, Label: "Character Sets"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := adb.GetAuth(inputs)
	if err != nil {
		return adb.ErrorResult(err.Error()), nil
	}
	client, err := auth.DatabaseClient()
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := adb.Context()

	req := database.ListAutonomousDatabaseCharacterSetsRequest{}
	if v := strings.TrimSpace(adb.OptionalString("character_set_type", inputs)); v != "" {
		req.CharacterSetType = database.ListAutonomousDatabaseCharacterSetsCharacterSetTypeEnum(v)
	}

	resp, err := client.ListAutonomousDatabaseCharacterSets(ctx, req)
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}

	sets := make([]string, 0, len(resp.Items))
	for i := range resp.Items {
		if name := adb.Str(resp.Items[i].Name); name != "" {
			sets = append(sets, name)
		}
	}

	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Found %d character set(s)", len(sets)),
		"character_sets": sets,
		"count":          len(sets),
		"success":        true,
	}, nil
}
