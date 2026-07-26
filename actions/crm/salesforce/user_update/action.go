// Package crm_salesforce_user_update changes an existing Salesforce user.
//
// The mover half of joiners-movers-leavers: a promotion changes the title and
// the role, a reorg changes the manager, a department transfer changes the
// profile. n8n cannot do any of it — its User resource is read-only — so this is
// net-new capability rather than parity.
//
// Everything here rests on one rule: a blank input means "leave that field
// alone", never "clear it". Salesforce treats an omitted field and an explicit
// null as different instructions, so every field goes through SetIfPresent. An
// update that posted every empty box would blank the half of the record the
// operator did not fill in — on a User record, that means wiping someone's
// manager, department and title because you wanted to change their job title.
package crm_salesforce_user_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update User"
	Description  = "Change a Salesforce user's profile, role, manager, job title or contact details. Only the fields you fill in are changed — the rest are left alone."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// maxAliasLength is Salesforce's hard cap on User.Alias.
const maxAliasLength = 8

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID", Placeholder: "0055f00000AbCdEAAV — every Salesforce user ID starts with 005", Required: true},
	{Name: "profile_id", Type: core.ConnectionTypeString, Label: "Profile", Placeholder: "00e5f000000AbCdAAK — the profile that decides what they can see and do"},
	{Name: "user_role_id", Type: core.ConnectionTypeString, Label: "Role", Placeholder: "00E5f000000AbCdEAK — sets who can see their records"},
	{Name: "manager_id", Type: core.ConnectionTypeString, Label: "Manager", Placeholder: "0055f00000AbCdEAAV — the user ID of their line manager"},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Jane"},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Smith"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email Address", Placeholder: "jane.smith@yourcompany.com — Salesforce emails the new address to confirm the change"},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Login Username", Placeholder: "jane.smith@yourcompany.com — changing this changes how they sign in, and Salesforce sends a confirmation email"},
	{Name: "alias", Type: core.ConnectionTypeString, Label: "Alias", Placeholder: "jsmith — up to 8 characters"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Job Title", Placeholder: "Senior Account Manager"},
	{Name: "department", Type: core.ConnectionTypeString, Label: "Department", Placeholder: "Sales"},
	{Name: "division", Type: core.ConnectionTypeString, Label: "Division", Placeholder: "UK & Ireland"},
	{Name: "company_name", Type: core.ConnectionTypeString, Label: "Company Name", Placeholder: "Acme Ltd"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "+44 20 7946 0100"},
	{Name: "mobile_phone", Type: core.ConnectionTypeString, Label: "Mobile", Placeholder: "+44 7700 900123"},
	{Name: "employee_number", Type: core.ConnectionTypeString, Label: "Employee Number", Placeholder: "EMP-00421 — from your HR system"},
	{Name: "federation_identifier", Type: core.ConnectionTypeString, Label: "Single Sign-On ID", Placeholder: "jane.smith@yourcompany.com — only if your org signs in through SSO"},
	{Name: "nickname", Type: core.ConnectionTypeString, Label: "Nickname", Placeholder: "jsmith — shown in Chatter"},
	{Name: "time_zone_sid_key", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "Europe/London"},
	{Name: "locale_sid_key", Type: core.ConnectionTypeString, Label: "Locale (date and number format)", Placeholder: "en_GB"},
	{Name: "language_locale_key", Type: core.ConnectionTypeString, Label: "Language", Placeholder: "en_US — Salesforce has no en_GB language"},
	{Name: "street", Type: core.ConnectionTypeString, Label: "Street", Placeholder: "1 High Street"},
	{Name: "city", Type: core.ConnectionTypeString, Label: "City", Placeholder: "London"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "County or State", Placeholder: "California — must match your org's State list, if it uses one"},
	{Name: "postal_code", Type: core.ConnectionTypeString, Label: "Postcode", Placeholder: "SW1A 1AA"},
	{Name: "country", Type: core.ConnectionTypeString, Label: "Country", Placeholder: "United Kingdom"},
	{Name: "is_active", Type: core.ConnectionTypeBoolean, Label: "Login Active", Placeholder: "Turn on to restore a returning colleague's access; use Deactivate User for leavers"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"UserPermissionsMarketingUser\":true,\"Custom_Cost_Centre__c\":\"CC-100\"}"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Updated User"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	userID, err := salesforce.RequiredString("user_id", inputs)
	if err != nil {
		return nil, err
	}
	if err := salesforce.ValidateRecordID(userID); err != nil {
		return nil, err
	}

	// Lookup IDs are checked locally so a profile NAME pasted into the profile
	// field fails in the editor rather than as an INVALID_CROSS_REFERENCE_KEY
	// halfway through a run. Checked in a fixed order so an operator who got two
	// of them wrong is told about the same one every run.
	lookupIDs := []struct {
		input string
		value string
	}{
		{"profile_id", salesforce.OptionalString("profile_id", inputs)},
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

	if alias := salesforce.OptionalString("alias", inputs); alias != "" && len([]rune(alias)) > maxAliasLength {
		return nil, fmt.Errorf("alias must be %d characters or fewer — Salesforce caps it; %q is %d", maxAliasLength, alias, len([]rune(alias)))
	}

	// SetIfPresent throughout: a blank input means "leave this field as it is",
	// not "clear it". An update that posted every empty box would wipe half the
	// record — the single most damaging thing this action could get wrong. To
	// genuinely clear a field, pass an explicit null via Additional Fields.
	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "ProfileId", "profile_id")
	salesforce.SetIfPresent(body, inputs, "UserRoleId", "user_role_id")
	salesforce.SetIfPresent(body, inputs, "ManagerId", "manager_id")
	salesforce.SetIfPresent(body, inputs, "FirstName", "first_name")
	salesforce.SetIfPresent(body, inputs, "LastName", "last_name")
	salesforce.SetIfPresent(body, inputs, "Email", "email")
	salesforce.SetIfPresent(body, inputs, "Username", "username")
	salesforce.SetIfPresent(body, inputs, "Alias", "alias")
	salesforce.SetIfPresent(body, inputs, "Title", "title")
	salesforce.SetIfPresent(body, inputs, "Department", "department")
	salesforce.SetIfPresent(body, inputs, "Division", "division")
	salesforce.SetIfPresent(body, inputs, "CompanyName", "company_name")
	salesforce.SetIfPresent(body, inputs, "Phone", "phone")
	salesforce.SetIfPresent(body, inputs, "MobilePhone", "mobile_phone")
	salesforce.SetIfPresent(body, inputs, "EmployeeNumber", "employee_number")
	salesforce.SetIfPresent(body, inputs, "FederationIdentifier", "federation_identifier")
	salesforce.SetIfPresent(body, inputs, "CommunityNickname", "nickname")
	salesforce.SetIfPresent(body, inputs, "TimeZoneSidKey", "time_zone_sid_key")
	salesforce.SetIfPresent(body, inputs, "LocaleSidKey", "locale_sid_key")
	salesforce.SetIfPresent(body, inputs, "LanguageLocaleKey", "language_locale_key")
	salesforce.SetIfPresent(body, inputs, "Street", "street")
	salesforce.SetIfPresent(body, inputs, "City", "city")
	salesforce.SetIfPresent(body, inputs, "State", "state")
	salesforce.SetIfPresent(body, inputs, "PostalCode", "postal_code")
	salesforce.SetIfPresent(body, inputs, "Country", "country")
	// Tri-state, so an untouched tick box does not silently reactivate a leaver.
	salesforce.SetBoolIfSet(body, inputs, "IsActive", "is_active")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field to change, or use Additional Fields")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "User", userID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers a successful update with 204 No Content, so there is no
	// record to return. Echo the ID plus exactly what was written: without it
	// nothing downstream can chain off the update, which is the most common
	// shape these flows take.
	record := map[string]interface{}{"Id": userID}
	for k, v := range body {
		record[k] = v
	}

	changed := salesforce.SortedKeys(body)
	return salesforce.RecordResult(userID, record, fmt.Sprintf("Updated user %s — changed %s", userID, strings.Join(changed, ", "))), nil
}
