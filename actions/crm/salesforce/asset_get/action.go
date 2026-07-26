package crm_salesforce_asset_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Asset"
	Description  = "Look up one asset by its Salesforce ID and return everything on it - which product it is, its serial number, install and purchase dates, when the warranty ends and every custom field."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "asset_id", Type: core.ConnectionTypeString, Label: "Asset ID", Placeholder: "02i5f000000AbCdAAK - the asset's Salesforce ID, not its serial number", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Name,SerialNumber,Status,InstallDate,UsageEndDate - leave blank to return every field"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Asset ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Asset"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// A malformed ID is a wiring mistake, not a Salesforce failure, so it is a hard
	// error — catching it here beats a MALFORMED_ID nobody can act on. The obvious
	// mistake on this object is pasting the serial number, which is the identifier
	// a person actually reads off the unit.
	id := salesforce.OptionalString("asset_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, fmt.Errorf("Asset ID — %w. A serial number is not a record ID; an asset's record ID starts with 02i. Use Get Many Assets with a Serial Number filter if that is all you have", err)
	}

	// Blank fields means "everything readable": Salesforce omits the ?fields=
	// filter and returns the full sObject, custom fields included. That is the
	// useful default here, because the field a support flow needs — a site code, a
	// maintenance contract reference — is very often a custom one.
	fields := salesforce.OptionalString("fields", inputs)
	// A misspelled field name — the label rather than the API name, which is the
	// commonest Salesforce mistake there is — is rejected locally and never
	// reaches Salesforce, so it is a configuration mistake and takes the hard
	// error return. Left to GetRecord it lands on the soft error port as though
	// Salesforce had refused a well-formed request, and the flow carries on down
	// its failure branch retrying a typo forever.
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, fmt.Errorf("Fields — %w", err)
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Asset", id, fields)
	if err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf("assets are not available in your Salesforce org — an administrator can switch the Assets tab and object permissions on, and some Salesforce editions do not include them at all (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Retrieved asset %s", id)
	if name, ok := record["Name"].(string); ok && name != "" {
		summary = fmt.Sprintf("Retrieved asset %q (%s)", name, id)
	}
	return salesforce.RecordResult(id, record, summary), nil
}
