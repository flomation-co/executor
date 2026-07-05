// Package ukgov_parliament_get_member retrieves a single UK Parliament member
// by ID. No authentication required.
package ukgov_parliament_get_member

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Member"
	Description  = "Retrieve a UK Parliament member (MP or Lord) by ID (UK Parliament)"
	Website      = "https://www.flomation.co"
	Icon         = "landmark+user"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var baseURL = "https://members-api.parliament.uk"

var Inputs = [...]core.Connection{
	{Name: "member_id", Type: core.ConnectionTypeString, Label: "Member ID", Placeholder: "e.g. 4514", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "member", Type: core.ConnectionTypeObject, Label: "Member"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "party", Type: core.ConnectionTypeString, Label: "Party"},
	{Name: "constituency", Type: core.ConnectionTypeString, Label: "Constituency / Seat"},
	{Name: "house", Type: core.ConnectionTypeString, Label: "House"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type party struct {
	Name string `json:"name"`
}

type houseMembership struct {
	MembershipFrom string `json:"membershipFrom"`
	House          int    `json:"house"`
}

type memberValue struct {
	ID                    int             `json:"id"`
	NameDisplayAs         string          `json:"nameDisplayAs"`
	NameFullTitle         string          `json:"nameFullTitle"`
	Gender                string          `json:"gender"`
	LatestParty           party           `json:"latestParty"`
	LatestHouseMembership houseMembership `json:"latestHouseMembership"`
}

type memberResponse struct {
	Value memberValue `json:"value"`
}

func houseName(h int) string {
	switch h {
	case 1:
		return "Commons"
	case 2:
		return "Lords"
	default:
		return ""
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	id, err := ukgov_common.RequiredString("member_id", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A member ID is required.")
	}

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	endpoint := baseURL + "/api/Members/" + url.PathEscape(id)
	status, body, err := ukgov_common.Fetch(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ukgov_common.ErrResult("UK Parliament request failed: %v", err)
	}
	if status == http.StatusNotFound {
		return ukgov_common.ErrResult("No UK Parliament member found with ID %s.", id)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("UK Parliament returned status %d", status)
	}

	var parsed memberResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse UK Parliament response: %v", err)
	}
	v := parsed.Value
	if v.ID == 0 && v.NameDisplayAs == "" {
		return ukgov_common.ErrResult("No UK Parliament member found with ID %s.", id)
	}

	house := houseName(v.LatestHouseMembership.House)
	role := "member"
	if v.LatestHouseMembership.House == 1 {
		role = "MP"
	} else if v.LatestHouseMembership.House == 2 {
		role = "Lord"
	}
	summary := fmt.Sprintf("%s — %s %s for %s.", v.NameDisplayAs, v.LatestParty.Name, role, v.LatestHouseMembership.MembershipFrom)

	return map[string]interface{}{
		"tool_result":  summary,
		"member":       v,
		"name":         v.NameDisplayAs,
		"party":        v.LatestParty.Name,
		"constituency": v.LatestHouseMembership.MembershipFrom,
		"house":        house,
		"success":      true,
		"error":        "",
	}, nil
}
