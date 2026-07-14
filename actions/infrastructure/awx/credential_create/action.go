// Package infrastructure_awx_credential_create stores a new credential in AWX.
//
// The shape of the Credential Fields object is decided entirely by the credential
// type: a Machine credential wants username/ssh_key_data, an AWS one wants
// username/password (access key / secret key), a Source Control one wants
// username/password or ssh_key_data. So this action does not — and cannot —
// enumerate them as inputs. It takes the object, and validates it against the
// chosen type's own schema BEFORE posting.
//
// Three AWX rules make that pre-validation worth the extra round-trip:
//
//   - ★ inputs.required IS NOT ENFORCED AT CREATE TIME. AWX will happily create a
//     credential missing a required field and only blow up later, at job runtime,
//     inside a playbook — which is about the worst place for a non-technical
//     operator to meet the error. We check it client-side and name the fields.
//   - Unknown keys ARE rejected (the input schema sets additionalProperties:false),
//     but as an opaque 400. Naming the valid fields is far more useful.
//   - "$encrypted$" is a RESERVED KEYWORD on create — it means "keep the existing
//     value", which on a brand-new credential is nonsense. Someone who pasted the
//     output of Get Credential straight into this field gets told exactly that.
//
// OWNERSHIP: AWX demands exactly one of organization / user / team (400 "Missing
// user, team, or organization." with none, 400 "Only one of…" with more than
// one). This action exposes organization only — the sane default, and it keeps
// the input count down.
//
// There is deliberately NO credential_update action: PATCHing `inputs` REPLACES
// the whole dict rather than merging into it, so an update that meant to change
// one field silently blanks every other. Delete and recreate instead.
package infrastructure_awx_credential_create

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
	Name         = "AWX: Create Credential"
	Description  = "Store a new credential in AWX — an SSH key, a cloud key, a source-control token."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+plus"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

// encryptedSentinel is AWX's "keep the existing value" keyword. It is only ever
// valid on an update, never on a create.
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

	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "What to call this credential in AWX, e.g. Deploy key (production)", Required: true},
	{Name: "credential_type_id", Type: core.ConnectionTypeString, Label: "Credential Type", Placeholder: "Machine, Amazon Web Services, Source Control … — this decides which Credential Fields are expected", Required: true},
	{Name: "organization_id", Type: core.ConnectionTypeString, Label: "Organization", Placeholder: "The organization that will own the credential", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional — what this credential is for"},
	{Name: "inputs", Type: core.ConnectionTypeObject, Label: "Credential Fields", Placeholder: `{"username":"deploy","ssh_key_data":"-----BEGIN…"} — the field names come from the credential type; see AWX ▸ Credential Types, or GET /api/v2/credential_types/{id}/`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Credential ID"},
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

	name, err := awx.RequiredString("name", "Name", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	typeID, err := awx.RequiredInt("credential_type_id", "Credential Type", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	orgID, err := awx.RequiredInt("organization_id", "Organization", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	fields, err := awx.OptionalJSONObject("inputs", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if len(fields) == 0 {
		return awx.ErrorResult(`Credential Fields is required — the values this credential stores, keyed by the credential type's field names, e.g. {"username":"deploy","password":"…"}`), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	// ★ Validate against the credential type's own schema before posting.
	credType, err := awx.GetResource(ctx, auth, fmt.Sprintf("credential_types/%d/", typeID), nil)
	if err != nil {
		return awx.ErrorResult(fmt.Sprintf("Could not read credential type %d, so the Credential Fields cannot be checked: %s", typeID, err.Error())), nil
	}
	if err := validateFields(credType, fields); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"name":            name,
		"credential_type": typeID,
		"organization":    orgID,
		"inputs":          fields,
	}
	awx.SetIfPresent(body, inputs, "description", "description")

	obj, err := awx.CreateResource(ctx, auth, "credentials/", body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	return awx.ObjectResult(obj, fmt.Sprintf(
		"Created credential %q (%s) of type %s in organization %d. AWX has encrypted its secret values and will never return them.",
		name, awx.IDString(obj["id"]), awx.StringField(credType, "name"), orgID)), nil
}

// validateFields checks the operator's Credential Fields against the credential
// type's input schema — inputs.fields (the allowed keys) and inputs.required (the
// mandatory ones, which AWX itself does NOT enforce on create).
//
// A type whose schema we cannot read (a custom type with no fields declared) is
// let through: AWX is the authority, and refusing on our own ignorance would be
// worse than letting AWX answer.
func validateFields(credType map[string]interface{}, given map[string]interface{}) error {
	schema, ok := credType["inputs"].(map[string]interface{})
	if !ok {
		return nil
	}

	allowed := map[string]bool{}
	order := []string{}
	if declared, ok := schema["fields"].([]interface{}); ok {
		for _, item := range declared {
			field, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if id := awx.StringField(field, "id"); id != "" {
				allowed[id] = true
				order = append(order, id)
			}
		}
	}

	problems := []string{}

	// "$encrypted$" means "keep the existing value" — meaningless on a create, and
	// AWX rejects it as a reserved keyword. It is what someone gets if they pipe
	// Get Credential's output straight back in.
	pasted := []string{}
	for field, value := range given {
		if s, ok := value.(string); ok && strings.TrimSpace(s) == encryptedSentinel {
			pasted = append(pasted, field)
		}
	}
	if len(pasted) > 0 {
		sort.Strings(pasted)
		return fmt.Errorf(
			`Credential Fields contains the literal text "%s" for %s. That is not a value — it is what AWX shows INSTEAD of a secret it will never hand back, and AWX rejects it when creating a credential. Put the real secret in, or leave the field out.`,
			encryptedSentinel, strings.Join(pasted, ", "))
	}

	if len(allowed) > 0 {
		unknown := []string{}
		for field := range given {
			if !allowed[field] {
				unknown = append(unknown, field)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			problems = append(problems, fmt.Sprintf(
				"%s is not a field of this credential type (AWX will reject it). The fields it accepts are: %s",
				strings.Join(unknown, ", "), strings.Join(order, ", ")))
		}
	}

	// ★ AWX does NOT enforce this on create — the credential would be saved and
	// then fail inside a playbook, hours later, with a far worse error.
	missing := []string{}
	for _, item := range requiredNames(schema["required"]) {
		if v, ok := given[item]; !ok || blank(v) {
			missing = append(missing, item)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		problems = append(problems, fmt.Sprintf(
			"%s %s required by this credential type. AWX would accept the credential without %s and only fail later, when a job tries to use it",
			strings.Join(missing, ", "), plural(len(missing)), pronoun(len(missing))))
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("Credential Fields does not match the chosen credential type: %s.", strings.Join(problems, "; "))
}

// requiredNames reads the credential type's inputs.required list — a JSON array
// of field names.
func requiredNames(v interface{}) []string {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func blank(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	default:
		return false
	}
}

func plural(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

func pronoun(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}
