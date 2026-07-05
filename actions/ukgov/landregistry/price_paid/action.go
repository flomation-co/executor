// Package ukgov_landregistry_price_paid looks up sold-property prices by
// postcode from HM Land Registry's Price Paid linked-data API (no auth).
package ukgov_landregistry_price_paid

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Price Paid"
	Description  = "Look up UK sold-property prices by postcode (HM Land Registry Price Paid)"
	Website      = "https://www.flomation.co"
	Icon         = "house+dollar-sign"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

// baseURL is the Land Registry linked-data root. Package variable so tests can
// point it at a mock server.
var baseURL = "https://landregistry.data.gov.uk"

var Inputs = [...]core.Connection{
	{Name: "postcode", Type: core.ConnectionTypeString, Label: "Postcode", Placeholder: "PL6 8RU", Required: true},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results (1-50)", Placeholder: "20"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "transactions", Type: core.ConnectionTypeObject, Label: "Transactions"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// codedLabel decodes Land Registry's coded fields (propertyType, estateType),
// which are objects with a human-readable label rather than plain strings.
type codedLabel struct {
	About string `json:"_about"`
	Label []struct {
		Value string `json:"_value"`
	} `json:"label"`
}

func (c codedLabel) Text() string {
	if len(c.Label) > 0 {
		return c.Label[0].Value
	}
	return ""
}

type propertyAddress struct {
	Paon     string `json:"paon"`
	Saon     string `json:"saon"`
	Street   string `json:"street"`
	Locality string `json:"locality"`
	Town     string `json:"town"`
	County   string `json:"county"`
	Postcode string `json:"postcode"`
}

func (a propertyAddress) OneLine() string {
	parts := make([]string, 0, 6)
	for _, p := range []string{a.Saon, a.Paon, a.Street, a.Locality, a.Town, a.Postcode} {
		if s := strings.TrimSpace(p); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

type transaction struct {
	PricePaid       int             `json:"pricePaid"`
	TransactionDate string          `json:"transactionDate"`
	NewBuild        bool            `json:"newBuild"`
	PropertyType    codedLabel      `json:"propertyType"`
	EstateType      codedLabel      `json:"estateType"`
	PropertyAddress propertyAddress `json:"propertyAddress"`
}

type ppdResponse struct {
	Result struct {
		Items []transaction `json:"items"`
	} `json:"result"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	pcInput, err := ukgov_common.RequiredString("postcode", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A postcode is required.")
	}
	postcode := normalisePostcode(pcInput)

	maxResults := ukgov_common.OptionalInt("max_results", inputs, 20)
	if maxResults <= 0 {
		maxResults = 20
	}
	if maxResults > 50 {
		maxResults = 50
	}

	q := url.Values{}
	q.Set("propertyAddress.postcode", postcode)
	q.Set("_pageSize", fmt.Sprintf("%d", maxResults))
	q.Set("_sort", "-transactionDate")
	endpoint := baseURL + "/data/ppi/transaction-record.json?" + q.Encode()

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := ukgov_common.Fetch(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ukgov_common.ErrResult("Land Registry request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Land Registry returned status %d", status)
	}

	var parsed ppdResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse Land Registry response: %v", err)
	}

	return map[string]interface{}{
		"tool_result":  summarise(parsed.Result.Items, postcode),
		"transactions": parsed.Result.Items,
		"count":        len(parsed.Result.Items),
		"success":      true,
		"error":        "",
	}, nil
}

// normalisePostcode uppercases and inserts the standard single space before the
// final three characters, as the Land Registry API requires.
func normalisePostcode(pc string) string {
	s := strings.ToUpper(strings.ReplaceAll(pc, " ", ""))
	if len(s) > 3 {
		return s[:len(s)-3] + " " + s[len(s)-3:]
	}
	return s
}

func summarise(txns []transaction, postcode string) string {
	if len(txns) == 0 {
		return fmt.Sprintf("No sold-property records found for %s.", postcode)
	}
	latest := txns[0]
	estate := latest.EstateType.Text()
	ptype := latest.PropertyType.Text()
	descBits := make([]string, 0, 2)
	if ptype != "" {
		descBits = append(descBits, ptype)
	}
	if estate != "" {
		descBits = append(descBits, estate)
	}
	desc := ""
	if len(descBits) > 0 {
		desc = " (" + strings.Join(descBits, ", ") + ")"
	}
	return fmt.Sprintf("%d sold-property record(s) for %s. Most recent: %s on %s%s — %s.",
		len(txns), postcode, formatPrice(latest.PricePaid), latest.TransactionDate, desc, latest.PropertyAddress.OneLine())
}

// formatPrice renders an integer pound amount with thousands separators.
func formatPrice(amount int) string {
	s := fmt.Sprintf("%d", amount)
	n := len(s)
	if n <= 3 {
		return "£" + s
	}
	var b strings.Builder
	b.WriteString("£")
	lead := n % 3
	if lead > 0 {
		b.WriteString(s[:lead])
		if n > lead {
			b.WriteString(",")
		}
	}
	for i := lead; i < n; i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < n {
			b.WriteString(",")
		}
	}
	return b.String()
}
