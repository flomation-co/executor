// Package oracle_logging_search_logs runs a Logging Query Language search over stored log content
// across the tenancy for a given time window, returning the matching log entries and their count.
package oracle_logging_search_logs

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	lg "flomation.app/automate/executor/actions/oracle/logging"

	ocicommon "github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/loggingsearch"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Logging: Search Logs"
	Description  = "Run a Logging Query Language search over stored log content for a time window, returning the matching log entries and their count."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+file-lines"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "search_query", Type: core.ConnectionTypeText, Label: "Search Query", Placeholder: "Logging Query Language, e.g. search \"ocid1.compartment.oc1..aaaa\" | sort by datetime desc", Required: true},
	{Name: "time_start", Type: core.ConnectionTypeString, Label: "Start Time", Placeholder: "RFC3339, e.g. 2026-07-22T09:00:00Z", Required: true},
	{Name: "time_end", Type: core.ConnectionTypeString, Label: "End Time", Placeholder: "RFC3339, e.g. 2026-07-22T12:00:00Z", Required: true},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max entries to return (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Results"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := lg.SearchClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	query, err := lg.RequiredString("search_query", inputs)
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}
	startRaw, err := lg.RequiredString("time_start", inputs)
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}
	endRaw, err := lg.RequiredString("time_end", inputs)
	if err != nil {
		return lg.ErrorResult(err.Error()), nil
	}
	start, perr := time.Parse(time.RFC3339, startRaw)
	if perr != nil {
		return lg.ErrorResult("start time must be RFC3339, e.g. 2026-07-22T09:00:00Z"), nil
	}
	end, perr := time.Parse(time.RFC3339, endRaw)
	if perr != nil {
		return lg.ErrorResult("end time must be RFC3339, e.g. 2026-07-22T12:00:00Z"), nil
	}

	details := loggingsearch.SearchLogsDetails{
		SearchQuery: &query,
		TimeStart:   &ocicommon.SDKTime{Time: start.UTC()},
		TimeEnd:     &ocicommon.SDKTime{Time: end.UTC()},
	}
	req := loggingsearch.SearchLogsRequest{SearchLogsDetails: details}
	if n, ok, ierr := lg.OptionalInt("limit", inputs); ierr != nil {
		return lg.ErrorResult(ierr.Error()), nil
	} else if ok {
		req.Limit = &n
	}

	resp, err := client.SearchLogs(lg.Context(), req)
	if err != nil {
		return lg.ErrorResult(auth.OCIError(err)), nil
	}

	rows := make([]interface{}, 0, len(resp.Results))
	for i := range resp.Results {
		if resp.Results[i].Data != nil {
			rows = append(rows, *resp.Results[i].Data)
		}
	}

	return lg.Result(fmt.Sprintf("Search returned %d result(s)", len(rows)), map[string]interface{}{
		"results": rows,
		"count":   fmt.Sprintf("%d", len(rows)),
	}), nil
}
