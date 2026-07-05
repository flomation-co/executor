// Package ukgov_dvla_vehicle_enquiry looks up a UK vehicle's tax, MOT status
// and details from the DVLA Vehicle Enquiry Service (VES).
//
// NOTE: at time of writing DVLA has paused new VES API registrations while it
// upgrades its systems, so a live api_key cannot currently be obtained. The
// action is complete and hermetically tested; it will work end-to-end once
// DVLA reopens registration and a key is available.
package ukgov_dvla_vehicle_enquiry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Vehicle Enquiry"
	Description  = "Look up a UK vehicle's tax, MOT status, make, colour and CO2 by registration (DVLA)"
	Website      = "https://www.flomation.co"
	Icon         = "car+magnifying-glass"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

// baseURL is the DVLA VES root. Package variable so tests can point it at a
// mock server.
var baseURL = "https://driver-vehicle-licensing.api.gov.uk"

const enquiryPath = "/vehicle-enquiry/v1/vehicles"

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "DVLA API Key", Placeholder: "${secrets.DVLA_VES_KEY}", Required: true},
	{Name: "registration_number", Type: core.ConnectionTypeString, Label: "Registration Number", Placeholder: "e.g. AB19 ABC", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vehicle", Type: core.ConnectionTypeObject, Label: "Vehicle"},
	{Name: "make", Type: core.ConnectionTypeString, Label: "Make"},
	{Name: "colour", Type: core.ConnectionTypeString, Label: "Colour"},
	{Name: "tax_status", Type: core.ConnectionTypeString, Label: "Tax Status"},
	{Name: "mot_status", Type: core.ConnectionTypeString, Label: "MOT Status"},
	{Name: "year_of_manufacture", Type: core.ConnectionTypeInteger, Label: "Year of Manufacture"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type vehicle struct {
	RegistrationNumber       string `json:"registrationNumber"`
	TaxStatus                string `json:"taxStatus"`
	TaxDueDate               string `json:"taxDueDate"`
	MotStatus                string `json:"motStatus"`
	MotExpiryDate            string `json:"motExpiryDate"`
	Make                     string `json:"make"`
	YearOfManufacture        int    `json:"yearOfManufacture"`
	EngineCapacity           int    `json:"engineCapacity"`
	Co2Emissions             int    `json:"co2Emissions"`
	FuelType                 string `json:"fuelType"`
	Colour                   string `json:"colour"`
	MonthOfFirstRegistration string `json:"monthOfFirstRegistration"`
	Wheelplan                string `json:"wheelplan"`
	TypeApproval             string `json:"typeApproval"`
	EuroStatus               string `json:"euroStatus"`
	MarkedForExport          bool   `json:"markedForExport"`
	DateOfLastV5CIssued      string `json:"dateOfLastV5CIssued"`
}

// dvlaErrors mirrors the VES error envelope. The 403 (bad key) path uses the
// API-Gateway {"message":"..."} form instead, so Message is decoded too.
type dvlaErrors struct {
	Errors []struct {
		Status string `json:"status"`
		Code   string `json:"code"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
	} `json:"errors"`
	Message string `json:"message"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := ukgov_common.RequiredString("api_key", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A DVLA API key is required.")
	}
	regInput, err := ukgov_common.RequiredString("registration_number", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A registration number is required.")
	}
	// DVLA expects the VRN uppercased with no spaces.
	reg := strings.ToUpper(strings.ReplaceAll(regInput, " ", ""))

	payload, _ := json.Marshal(map[string]string{"registrationNumber": reg})

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := ukgov_common.FetchWithBody(ctx, http.MethodPost, baseURL+enquiryPath,
		map[string]string{"x-api-key": apiKey}, payload)
	if err != nil {
		return ukgov_common.ErrResult("DVLA request failed: %v", err)
	}

	switch status {
	case http.StatusOK:
		// handled below
	case http.StatusNotFound:
		return ukgov_common.ErrResult("No vehicle found with registration %s.", reg)
	case http.StatusBadRequest:
		return ukgov_common.ErrResult("Invalid registration number %q: %s", reg, decodeError(body))
	case http.StatusForbidden:
		return ukgov_common.ErrResult("DVLA authentication failed — check the API key.")
	default:
		return ukgov_common.ErrResult("DVLA returned status %d: %s", status, decodeError(body))
	}

	var v vehicle
	if err := json.Unmarshal(body, &v); err != nil {
		return ukgov_common.ErrResult("Failed to parse DVLA response: %v", err)
	}

	return map[string]interface{}{
		"tool_result":         summarise(v),
		"vehicle":             v,
		"make":                v.Make,
		"colour":              v.Colour,
		"tax_status":          v.TaxStatus,
		"mot_status":          v.MotStatus,
		"year_of_manufacture": v.YearOfManufacture,
		"success":             true,
		"error":               "",
	}, nil
}

// decodeError extracts a human-readable message from a VES error body, trying
// the errors[] envelope first and falling back to the gateway message form.
func decodeError(body []byte) string {
	var e dvlaErrors
	if err := json.Unmarshal(body, &e); err == nil {
		if len(e.Errors) > 0 {
			if e.Errors[0].Detail != "" {
				return e.Errors[0].Detail
			}
			if e.Errors[0].Title != "" {
				return e.Errors[0].Title
			}
		}
		if e.Message != "" {
			return e.Message
		}
	}
	return "unknown error"
}

// summarise builds a concise, AI-readable one-line vehicle summary.
func summarise(v vehicle) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s %s.", v.RegistrationNumber, strings.TrimSpace(v.Colour), strings.TrimSpace(v.Make))
	if v.TaxStatus != "" {
		fmt.Fprintf(&b, " Tax: %s", v.TaxStatus)
		if v.TaxDueDate != "" {
			fmt.Fprintf(&b, " (due %s)", v.TaxDueDate)
		}
		b.WriteString(".")
	}
	if v.MotStatus != "" {
		fmt.Fprintf(&b, " MOT: %s", v.MotStatus)
		if v.MotExpiryDate != "" {
			fmt.Fprintf(&b, " (expires %s)", v.MotExpiryDate)
		}
		b.WriteString(".")
	}
	if v.YearOfManufacture != 0 {
		fmt.Fprintf(&b, " %d", v.YearOfManufacture)
		if v.FuelType != "" {
			fmt.Fprintf(&b, ", %s", v.FuelType)
		}
		if v.Co2Emissions != 0 {
			fmt.Fprintf(&b, ", %d g/km CO2", v.Co2Emissions)
		}
		b.WriteString(".")
	}
	return b.String()
}
