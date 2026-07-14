// Package infrastructure_awx_credential_get fetches one credential's non-secret
// details.
//
// ★ AWX NEVER RETURNS A STORED SECRET. Credential.display_inputs() replaces the
// value of every input the credential type marks secret (password, ssh_key_data,
// vault_password, token, client_secret…) with the LITERAL STRING "$encrypted$".
// So `result.inputs.password` is the eleven characters "$encrypted$" — it is
// AWX's placeholder, not the password, and not something this node redacted.
//
// A flow that wired that value into an SSH step would try to authenticate with
// the word "$encrypted$". This action therefore lists the encrypted field names
// in its summary in so many words, and NEVER presents "$encrypted$" as if it
// were a real value.
package infrastructure_awx_credential_get

import (
	"fmt"
	"sort"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Credential"
	Description  = "Fetch one credential's non-secret details. Secret fields come back as $encrypted$ — AWX never returns a stored secret."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+eye"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

// encryptedSentinel is what AWX puts in place of every secret input value.
const encryptedSentinel = "$encrypted$"

var Inputs = [...]core.Connection{
	{Name: "awx_url", Type: core.ConnectionTypeString, Label: "AWX / AAP URL", Placeholder: "https://awx.example.com — your AWX or Ansible Automation Platform address", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{
		{Name: "API Token (recommended)", Value: "token"},
		{Name: "Username & Password", Value: "basic"},
	}},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "AWX ▸ your user ▸ Tokens ▸ Add, Application blank, Scope = Write. Shown once — copy it then.", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "token"}}},
	{Name: "awx_username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your AWX username", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"basic"}}},
	{Name: "awx_password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "your AWX password — note some AWX installs disable password authentication", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"basic"}}},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip certificate verification — only for a self-hosted AWX with a self-signed certificate"},
	{Name: "api_prefix", Type: core.ConnectionTypeString, Label: "API Path Prefix (advanced)", Placeholder: "Leave blank — detected automatically. Only set this if support asks (e.g. /api/controller/v2/)."},

	{Name: "credential_id", Type: core.ConnectionTypeString, Label: "Credential", Placeholder: "The credential to fetch — its secret values are never returned", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Credential ID"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "kind", Type: core.ConnectionTypeString, Label: "Kind"},
	{Name: "credential_type", Type: core.ConnectionTypeString, Label: "Credential Type"},
	{Name: "managed", Type: core.ConnectionTypeBoolean, Label: "Managed by AWX"},
	{Name: "encrypted_fields", Type: core.ConnectionTypeObject, Label: "Encrypted Field Names"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Credential"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	credentialID, err := awx.RequiredInt("credential_id", "Credential", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	obj, err := awx.GetResource(ctx, auth, fmt.Sprintf("credentials/%d/", credentialID), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	name := awx.StringField(obj, "name")
	kind := awx.StringField(obj, "kind")
	typeName := credentialTypeName(obj)
	managed := awx.BoolField(obj, "managed")
	encrypted := encryptedFields(obj)

	summary := fmt.Sprintf("Credential %q (%s)", name, awx.IDString(obj["id"]))
	if typeName != "" {
		summary += " — type " + typeName
	}
	if managed {
		summary += ". It is managed by AWX, so it cannot be edited or deleted"
	}
	if len(encrypted) > 0 {
		summary += fmt.Sprintf(
			". AWX did not return the secret value of %s — each reads back as the literal text \"%s\", which is AWX's placeholder, NOT the stored secret. Do not feed it into anything that expects the real value.",
			strings.Join(encrypted, ", "), encryptedSentinel)
	} else {
		summary += ". It has no secret fields"
	}

	out := awx.ObjectResult(obj, summary)
	out["name"] = name
	out["kind"] = kind
	out["credential_type"] = typeName
	out["managed"] = managed
	out["encrypted_fields"] = encrypted
	return out, nil
}

// credentialTypeName prefers the human name AWX puts in summary_fields, falling
// back to the bare type id so the output is never empty.
func credentialTypeName(obj map[string]interface{}) string {
	if summary, ok := obj["summary_fields"].(map[string]interface{}); ok {
		if ct, ok := summary["credential_type"].(map[string]interface{}); ok {
			if name := awx.StringField(ct, "name"); name != "" {
				return name
			}
		}
	}
	return awx.StringField(obj, "credential_type")
}

// encryptedFields lists, sorted, the input names whose value AWX replaced with
// the $encrypted$ sentinel.
func encryptedFields(obj map[string]interface{}) []string {
	values, ok := obj["inputs"].(map[string]interface{})
	if !ok {
		return []string{}
	}
	out := []string{}
	for field, value := range values {
		if s, ok := value.(string); ok && s == encryptedSentinel {
			out = append(out, field)
		}
	}
	sort.Strings(out)
	return out
}
