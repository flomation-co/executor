// Package crm_salesforce_user_create provisions a Salesforce login for a new
// starter.
//
// n8n's Salesforce node is read-only on User — get and get-many, nothing else —
// so joiner automation stops at "someone in IT clicks New User". This closes
// that gap: an HR system's new-starter record becomes a working Salesforce
// account with the right profile, role and manager attached.
//
// Two things make User harder to create than any other sObject, and both are
// handled here rather than left to the operator:
//
//   - It has NINE required fields, five of which (alias, time zone, locale,
//     language, email encoding) are Salesforce platform trivia that no HR system
//     records. Sensible defaults are applied so a joiner flow does not fail on
//     an email-encoding key.
//
//   - Username is not the email address. It must LOOK like one, but it is a
//     login, it is unique across every Salesforce org on earth, and it is
//     conventionally suffixed (jane.smith@acme.com.crm) precisely so it does not
//     clash. Getting that wrong is the single most common failure, so the check
//     and the explanation happen before the request is sent.
//
// Requires the Manage Users permission and consumes a user licence.
package crm_salesforce_user_create

import (
	"fmt"
	"strings"
	"unicode"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create User"
	Description  = "Set up a new starter's Salesforce login from an HR record — name, email, profile, role and manager. Needs the Manage Users permission and a spare user licence."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// Salesforce refuses to create a User without all of Username, LastName, Email,
// Alias, ProfileId, TimeZoneSidKey, LocaleSidKey, LanguageLocaleKey and
// EmailEncodingKey. The last four are Salesforce trivia an HR system will never
// supply, so rather than fail a new-starter flow on them these defaults are
// applied when the operator leaves the field blank. They are UK-shaped because
// that is who this ships to; anything else is one field away.
//
// Note that language and locale are NOT the same key space, and mixing them up
// is the classic first-run failure: "en_GB" is a valid LOCALE (date, number and
// currency formats) but not a valid LANGUAGE — Salesforce's English is en_US.
const (
	defaultTimeZone      = "Europe/London"
	defaultLocale        = "en_GB"
	defaultLanguage      = "en_US"
	defaultEmailEncoding = "UTF-8"

	// maxAliasLength is Salesforce's hard cap on User.Alias.
	maxAliasLength = 8
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Login Username", Placeholder: "jane.smith@yourcompany.com — looks like an email but is a login, and must be unique across all of Salesforce", Required: true},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Smith", Required: true},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Jane"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email Address", Placeholder: "jane.smith@yourcompany.com — where their welcome and password emails go", Required: true},
	{Name: "profile_id", Type: core.ConnectionTypeString, Label: "Profile", Placeholder: "00e5f000000AbCdAAK — the profile that decides what they can see and do", Required: true},
	{Name: "alias", Type: core.ConnectionTypeString, Label: "Alias", Placeholder: "jsmith — up to 8 characters; built from their name if you leave this blank"},
	{Name: "user_role_id", Type: core.ConnectionTypeString, Label: "Role", Placeholder: "00E5f000000AbCdEAK — sets who can see their records (optional)"},
	{Name: "manager_id", Type: core.ConnectionTypeString, Label: "Manager", Placeholder: "0055f00000AbCdEAAV — the user ID of their line manager (optional)"},
	{Name: "time_zone_sid_key", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "Europe/London (used if left blank)"},
	{Name: "locale_sid_key", Type: core.ConnectionTypeString, Label: "Locale (date and number format)", Placeholder: "en_GB (used if left blank)"},
	{Name: "language_locale_key", Type: core.ConnectionTypeString, Label: "Language", Placeholder: "en_US (used if left blank) — Salesforce has no en_GB language"},
	{
		Name:        "email_encoding_key",
		Type:        core.ConnectionTypeString,
		Label:       "Email Encoding",
		Placeholder: "UTF-8 (used if left blank)",
		Options: []core.ConnectionOption{
			{Name: "Unicode (UTF-8)", Value: "UTF-8"},
			{Name: "General US & Western Europe (ISO-8859-1)", Value: "ISO-8859-1"},
			{Name: "Japanese (Shift-JIS)", Value: "Shift_JIS"},
			{Name: "Japanese (JIS)", Value: "ISO-2022-JP"},
			{Name: "Japanese (EUC)", Value: "EUC-JP"},
			{Name: "Korean (ks_c_5601-1987)", Value: "ks_c_5601-1987"},
			{Name: "Traditional Chinese (Big5)", Value: "Big5"},
			{Name: "Simplified Chinese (GB2312)", Value: "GB2312"},
			{Name: "Traditional Chinese Hong Kong (Big5-HKSCS)", Value: "Big5-HKSCS"},
		},
	},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Job Title", Placeholder: "Account Manager"},
	{Name: "department", Type: core.ConnectionTypeString, Label: "Department", Placeholder: "Sales"},
	{Name: "division", Type: core.ConnectionTypeString, Label: "Division", Placeholder: "UK & Ireland"},
	{Name: "company_name", Type: core.ConnectionTypeString, Label: "Company Name", Placeholder: "Acme Ltd"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "+44 20 7946 0100"},
	{Name: "mobile_phone", Type: core.ConnectionTypeString, Label: "Mobile", Placeholder: "+44 7700 900123"},
	{Name: "employee_number", Type: core.ConnectionTypeString, Label: "Employee Number", Placeholder: "EMP-00421 — from your HR system"},
	{Name: "federation_identifier", Type: core.ConnectionTypeString, Label: "Single Sign-On ID", Placeholder: "jane.smith@yourcompany.com — only if your org signs in through SSO"},
	{Name: "nickname", Type: core.ConnectionTypeString, Label: "Nickname", Placeholder: "jsmith — shown in Chatter; generated automatically if left blank"},
	{Name: "street", Type: core.ConnectionTypeString, Label: "Street", Placeholder: "1 High Street"},
	{Name: "city", Type: core.ConnectionTypeString, Label: "City", Placeholder: "London"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "County or State", Placeholder: "Greater London"},
	{Name: "postal_code", Type: core.ConnectionTypeString, Label: "Postcode", Placeholder: "SW1A 1AA"},
	{Name: "country", Type: core.ConnectionTypeString, Label: "Country", Placeholder: "United Kingdom"},
	{Name: "is_active", Type: core.ConnectionTypeBoolean, Label: "Login Active", Placeholder: "On by default — turn off to create the account without letting them sign in yet"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"UserPermissionsMarketingUser\":true,\"Custom_Cost_Centre__c\":\"CC-100\"}"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Created User"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	username, err := salesforce.RequiredString("username", inputs)
	if err != nil {
		return nil, err
	}
	if err := validateUsername(username); err != nil {
		return nil, err
	}

	lastName, err := salesforce.RequiredString("last_name", inputs)
	if err != nil {
		return nil, err
	}
	email, err := salesforce.RequiredString("email", inputs)
	if err != nil {
		return nil, err
	}
	profileID, err := salesforce.RequiredString("profile_id", inputs)
	if err != nil {
		return nil, err
	}

	firstName := salesforce.OptionalString("first_name", inputs)
	alias, err := resolveAlias(salesforce.OptionalString("alias", inputs), firstName, lastName)
	if err != nil {
		return nil, err
	}

	// Every ID field is checked locally so a value pasted from the wrong column
	// (a profile NAME, an email address) fails while the operator is still in the
	// editor, rather than as an INVALID_CROSS_REFERENCE_KEY at three in the
	// morning when the joiner flow actually runs. Checked in a fixed order so an
	// operator who got two of them wrong is told about the same one every run.
	lookupIDs := []struct {
		input string
		value string
	}{
		{"profile_id", profileID},
		{"user_role_id", salesforce.OptionalString("user_role_id", inputs)},
		{"manager_id", salesforce.OptionalString("manager_id", inputs)},
	}
	for _, lookup := range lookupIDs {
		if lookup.value == "" {
			continue
		}
		if err := salesforce.ValidateRecordID(lookup.value); err != nil {
			return nil, fmt.Errorf("%s: %w", lookup.input, err)
		}
	}

	body := map[string]interface{}{
		"Username":  username,
		"LastName":  lastName,
		"Email":     email,
		"Alias":     alias,
		"ProfileId": profileID,
		// Required by Salesforce, defaulted here so the operator does not have to
		// know they exist. See the const block for why language != locale.
		"TimeZoneSidKey":    orDefault(salesforce.OptionalString("time_zone_sid_key", inputs), defaultTimeZone),
		"LocaleSidKey":      orDefault(salesforce.OptionalString("locale_sid_key", inputs), defaultLocale),
		"LanguageLocaleKey": orDefault(salesforce.OptionalString("language_locale_key", inputs), defaultLanguage),
		"EmailEncodingKey":  orDefault(salesforce.OptionalString("email_encoding_key", inputs), defaultEmailEncoding),
	}

	// SetIfPresent, never a plain assignment: an omitted field and an empty one
	// are different things to Salesforce, and sending "" for, say, ManagerId is
	// not the same as not mentioning it.
	salesforce.SetIfPresent(body, inputs, "FirstName", "first_name")
	salesforce.SetIfPresent(body, inputs, "UserRoleId", "user_role_id")
	salesforce.SetIfPresent(body, inputs, "ManagerId", "manager_id")
	salesforce.SetIfPresent(body, inputs, "Title", "title")
	salesforce.SetIfPresent(body, inputs, "Department", "department")
	salesforce.SetIfPresent(body, inputs, "Division", "division")
	salesforce.SetIfPresent(body, inputs, "CompanyName", "company_name")
	salesforce.SetIfPresent(body, inputs, "Phone", "phone")
	salesforce.SetIfPresent(body, inputs, "MobilePhone", "mobile_phone")
	salesforce.SetIfPresent(body, inputs, "EmployeeNumber", "employee_number")
	salesforce.SetIfPresent(body, inputs, "FederationIdentifier", "federation_identifier")
	salesforce.SetIfPresent(body, inputs, "CommunityNickname", "nickname")
	salesforce.SetIfPresent(body, inputs, "Street", "street")
	salesforce.SetIfPresent(body, inputs, "City", "city")
	salesforce.SetIfPresent(body, inputs, "State", "state")
	salesforce.SetIfPresent(body, inputs, "PostalCode", "postal_code")
	salesforce.SetIfPresent(body, inputs, "Country", "country")
	// Tri-state: untouched means "let Salesforce default it" (which is active),
	// so only an explicitly set tick box is sent.
	salesforce.SetBoolIfSet(body, inputs, "IsActive", "is_active")

	// Every Salesforce org has fields this action does not name — a custom cost
	// centre, a permission flag, a managed-package field — so the escape hatch is
	// the normal path here, not an edge case.
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, _, err := salesforce.CreateRecord(instanceURL, token, "User", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// The raw create response is deliberately dropped: it carries only
	// {id, success, errors} and the ID is already in hand. What a joiner flow's
	// next step actually wants is the username, alias and manager it just set,
	// so return those rather than making it re-read the record.
	record := map[string]interface{}{"Id": id}
	for k, v := range body {
		record[k] = v
	}

	who := strings.TrimSpace(firstName + " " + lastName)
	return salesforce.RecordResult(id, record, fmt.Sprintf("Created Salesforce user %s (%s) with alias %s", who, username, alias)), nil
}

// validateUsername checks the shape of a Salesforce login before the API does.
//
// The rule catches everyone out once: a Salesforce Username must LOOK like an
// email address but is not a mailbox, and it has to be unique across every
// Salesforce org in the world — which is why most companies append a suffix
// (jane.smith@yourcompany.com.crm). Saying that here is far more use than
// Salesforce's own "Username must be in the form of an email address".
func validateUsername(username string) error {
	at := strings.Index(username, "@")
	if at <= 0 || at == len(username)-1 || strings.ContainsAny(username, " \t") {
		return fmt.Errorf("username must be in email form, e.g. jane.smith@yourcompany.com — it is a Salesforce login rather than a mailbox, and it must be unique across every Salesforce org (many companies add a suffix such as .crm)")
	}
	return nil
}

// resolveAlias returns the Alias to send, deriving one from the name when the
// operator left the field blank.
//
// Alias is required by Salesforce, capped at 8 characters, and means nothing to
// the person setting up the account — an HR record will never carry it. The
// derived form is the familiar initial-plus-surname ("jsmith"), truncated to fit.
// An alias the operator DID type is never silently truncated: that would quietly
// create a user under a name they did not choose.
func resolveAlias(alias, firstName, lastName string) (string, error) {
	if alias != "" {
		if len([]rune(alias)) > maxAliasLength {
			return "", fmt.Errorf("alias must be %d characters or fewer — Salesforce caps it; %q is %d", maxAliasLength, alias, len([]rune(alias)))
		}
		return alias, nil
	}

	derived := ""
	if initial := firstLetter(firstName); initial != "" {
		derived = initial
	}
	derived += lettersOnly(lastName)
	if runes := []rune(derived); len(runes) > maxAliasLength {
		derived = string(runes[:maxAliasLength])
	}
	if derived == "" {
		// A name made entirely of punctuation or symbols leaves nothing to build
		// from; ask rather than invent something the operator cannot predict.
		return "", fmt.Errorf("alias could not be built from the name — set an alias of up to %d characters", maxAliasLength)
	}
	return strings.ToLower(derived), nil
}

// firstLetter returns the first letter or digit of a name, or "".
func firstLetter(s string) string {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(r)
		}
	}
	return ""
}

// lettersOnly strips spaces, hyphens, apostrophes and anything else Salesforce
// would rather not see in an alias (O'Brien, Smith-Jones).
func lettersOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// orDefault returns value, or fallback when the operator left the input blank.
func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
