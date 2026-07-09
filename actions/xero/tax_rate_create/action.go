package tax_rate_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Tax Rate: Create"
	Description  = "Create a Xero tax rate with its tax components. Returns the tax rate object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+calculator"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "20% VAT on Income", Required: true},
	{Name: "report_tax_type", Type: core.ConnectionTypeString, Label: "Report Tax Type", Placeholder: "OUTPUT2"},
	{Name: "tax_components", Type: core.ConnectionTypeText, Label: "Tax Components (JSON array)", Placeholder: `[{"Name":"VAT","Rate":20.0}]`},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"Status":"ACTIVE"}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	name, err := xero_common.RequiredString("name", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"Name": name}
	xero_common.SetString(body, "ReportTaxType", "report_tax_type", inputs)

	components, err := xero_common.ParseJSONArray("tax_components", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	if components != nil {
		body["TaxComponents"] = components
	}

	extra, err := xero_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	xero_common.MergeFields(body, extra)

	resp, err := xero_common.DoJSON(flow, "POST", "/TaxRates", token, tenant, body)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "TaxRates")
	return xero_common.ObjectResult("", obj, fmt.Sprintf("Created Xero tax rate %q", name)), nil
}
