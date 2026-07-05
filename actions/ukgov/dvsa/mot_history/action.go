// Package ukgov_dvsa_mot_history retrieves a UK vehicle's full MOT test history
// from the DVSA MOT History API.
//
// Auth is OAuth2 client-credentials via Microsoft Entra ID: the action exchanges
// client_id/client_secret for a bearer token (once per execution), then calls
// the vehicle endpoint with that token plus the issued X-API-Key. Credentials
// are obtained by registering with DVSA (1-5 working day approval).
package ukgov_dvsa_mot_history

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/dvsa"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "MOT History"
	Description  = "Retrieve a UK vehicle's full MOT test history by registration (DVSA)"
	Website      = "https://www.flomation.co"
	Icon         = "wrench+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "client_id", Type: core.ConnectionTypeSecret, Label: "DVSA Client ID", Placeholder: "${secrets.DVSA_CLIENT_ID}", Required: true},
	{Name: "client_secret", Type: core.ConnectionTypeSecret, Label: "DVSA Client Secret", Placeholder: "${secrets.DVSA_CLIENT_SECRET}", Required: true},
	{Name: "tenant_id", Type: core.ConnectionTypeString, Label: "DVSA Tenant ID", Placeholder: "${secrets.DVSA_TENANT_ID}", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "DVSA API Key", Placeholder: "${secrets.DVSA_API_KEY}", Required: true},
	{Name: "registration_number", Type: core.ConnectionTypeString, Label: "Registration Number", Placeholder: "e.g. AB19 ABC", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vehicle", Type: core.ConnectionTypeObject, Label: "Vehicle"},
	{Name: "make", Type: core.ConnectionTypeString, Label: "Make"},
	{Name: "model", Type: core.ConnectionTypeString, Label: "Model"},
	{Name: "mot_tests", Type: core.ConnectionTypeObject, Label: "MOT Tests"},
	{Name: "latest_result", Type: core.ConnectionTypeString, Label: "Latest Result"},
	{Name: "latest_expiry", Type: core.ConnectionTypeString, Label: "Latest Expiry"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type rfr struct {
	Text      string `json:"text"`
	Type      string `json:"type"`
	Dangerous bool   `json:"dangerous"`
}

type motTest struct {
	CompletedDate  string `json:"completedDate"`
	TestResult     string `json:"testResult"`
	ExpiryDate     string `json:"expiryDate"`
	OdometerValue  string `json:"odometerValue"`
	OdometerUnit   string `json:"odometerUnit"`
	MotTestNumber  string `json:"motTestNumber"`
	RfrAndComments []rfr  `json:"rfrAndComments"`
}

type vehicle struct {
	Registration  string    `json:"registration"`
	Make          string    `json:"make"`
	Model         string    `json:"model"`
	FirstUsedDate string    `json:"firstUsedDate"`
	FuelType      string    `json:"fuelType"`
	PrimaryColour string    `json:"primaryColour"`
	MotTests      []motTest `json:"motTests"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	clientID, err := ukgov_common.RequiredString("client_id", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A DVSA client ID is required.")
	}
	clientSecret, err := ukgov_common.RequiredString("client_secret", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A DVSA client secret is required.")
	}
	tenantID, err := ukgov_common.RequiredString("tenant_id", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A DVSA tenant ID is required.")
	}
	apiKey, err := ukgov_common.RequiredString("api_key", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A DVSA API key is required.")
	}
	regInput, err := ukgov_common.RequiredString("registration_number", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A registration number is required.")
	}
	reg := strings.ToUpper(strings.ReplaceAll(regInput, " ", ""))

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	tokenURL := dvsa.LoginBaseURL + "/" + url.PathEscape(tenantID) + "/oauth2/v2.0/token"
	token, err := ukgov_common.ClientCredentialsToken(ctx, tokenURL, clientID, clientSecret, dvsa.Scope)
	if err != nil {
		return ukgov_common.ErrResult("DVSA authentication failed: %v", err)
	}

	endpoint := dvsa.BaseURL + "/v1/trade/vehicles/registration/" + url.PathEscape(reg)
	status, body, err := ukgov_common.Fetch(ctx, http.MethodGet, endpoint, map[string]string{
		"Authorization": "Bearer " + token.AccessToken,
		"X-API-Key":     apiKey,
	})
	if err != nil {
		return ukgov_common.ErrResult("DVSA request failed: %v", err)
	}
	if status == http.StatusNotFound {
		return ukgov_common.ErrResult("No MOT history found for registration %s.", reg)
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return ukgov_common.ErrResult("DVSA authorisation failed — check the API key and credentials.")
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("DVSA returned status %d", status)
	}

	var v vehicle
	if err := json.Unmarshal(body, &v); err != nil {
		return ukgov_common.ErrResult("Failed to parse DVSA response: %v", err)
	}

	latestResult, latestExpiry := "", ""
	if len(v.MotTests) > 0 {
		latestResult = v.MotTests[0].TestResult
		latestExpiry = v.MotTests[0].ExpiryDate
	}

	return map[string]interface{}{
		"tool_result":   summarise(v),
		"vehicle":       v,
		"make":          v.Make,
		"model":         v.Model,
		"mot_tests":     v.MotTests,
		"latest_result": latestResult,
		"latest_expiry": latestExpiry,
		"success":       true,
		"error":         "",
	}, nil
}

func summarise(v vehicle) string {
	var b strings.Builder
	descr := strings.TrimSpace(strings.TrimSpace(v.Make + " " + v.Model))
	fmt.Fprintf(&b, "%s: %s", v.Registration, descr)
	extras := make([]string, 0, 2)
	if v.FuelType != "" {
		extras = append(extras, v.FuelType)
	}
	if v.PrimaryColour != "" {
		extras = append(extras, v.PrimaryColour)
	}
	if len(extras) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(extras, ", "))
	}
	b.WriteString(".")

	if len(v.MotTests) == 0 {
		b.WriteString(" No MOT tests on record.")
		return b.String()
	}
	latest := v.MotTests[0]
	fmt.Fprintf(&b, " Latest MOT: %s on %s", latest.TestResult, dateOnly(latest.CompletedDate))
	if latest.ExpiryDate != "" {
		fmt.Fprintf(&b, " (expires %s)", dateOnly(latest.ExpiryDate))
	}
	if latest.OdometerValue != "" {
		fmt.Fprintf(&b, ", %s %s", latest.OdometerValue, latest.OdometerUnit)
	}
	fmt.Fprintf(&b, ". %d test(s) on record.", len(v.MotTests))
	return b.String()
}

func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
