// Package product_update implements the Freshsales "Product: Update" action.
package product_update

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	freshsales_common "flomation.app/automate/executor/actions/crm/freshsales"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Product: Update"
	Description  = "Update a Freshsales CPQ product."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+pencil"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product ID", Placeholder: "12345", Required: true},
	{Name: "sku_number", Type: core.ConnectionTypeString, Label: "SKU", Placeholder: "FLO-GROW"},
	{Name: "category", Type: core.ConnectionTypeString, Label: "Category", Placeholder: "Software"},
	{Name: "base_currency_amount", Type: core.ConnectionTypeString, Label: "Base Price", Placeholder: "149.99"},
	{Name: "currency_id", Type: core.ConnectionTypeInteger, Label: "Currency ID"},
	{Name: "owner_id", Type: core.ConnectionTypeInteger, Label: "Owner ID"},
	{Name: "active", Type: core.ConnectionTypeBoolean, Label: "Active"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON)", Placeholder: `{"custom_field":{"cf_region":"EMEA"}}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Record"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	client, err := freshsales_common.Client(inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	idValue, err := freshsales_common.RequiredString("id", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	record := map[string]interface{}{}
	freshsales_common.SetString(record, "sku_number", "sku_number", inputs)
	freshsales_common.SetString(record, "category", "category", inputs)
	freshsales_common.SetNumber(record, "base_currency_amount", "base_currency_amount", inputs)
	freshsales_common.SetInt(record, "currency_id", "currency_id", inputs)
	freshsales_common.SetInt(record, "owner_id", "owner_id", inputs)
	freshsales_common.SetBool(record, "active", "active", inputs)
	freshsales_common.SetString(record, "description", "description", inputs)
	extra, err := freshsales_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	freshsales_common.MergeFields(record, extra)
	payload := map[string]interface{}{"product": record}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPut, "/cpq/products/"+idValue, payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	recordOut := freshsales_common.Obj(resp, "product")
	if recordOut == nil {
		recordOut = resp
	}
	return freshsales_common.ObjectResult(recordOut, fmt.Sprintf("Updated product %s", freshsales_common.NameOf(recordOut))), nil
}
