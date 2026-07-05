// Package ukgov_police_list_forces lists the UK territorial police forces
// available in the data.police.uk API. No authentication required.
package ukgov_police_list_forces

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
	"flomation.app/automate/executor/actions/ukgov/police"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Police Forces"
	Description  = "List UK territorial police forces and their IDs (Police UK)"
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "forces", Type: core.ConnectionTypeObject, Label: "Forces"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type force struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := police.Get(ctx, "/forces", nil)
	if err != nil {
		return ukgov_common.ErrResult("Police UK request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("Police UK returned status %d", status)
	}

	var forces []force
	if err := json.Unmarshal(body, &forces); err != nil {
		return ukgov_common.ErrResult("Failed to parse Police UK response: %v", err)
	}

	names := make([]string, 0, len(forces))
	for _, f := range forces {
		names = append(names, f.Name)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%d UK police forces available: %s.", len(forces), strings.Join(names, ", ")),
		"forces":      forces,
		"count":       len(forces),
		"success":     true,
		"error":       "",
	}, nil
}
