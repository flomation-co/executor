// Package oracle_nosql_table_usage_list reports throughput and storage usage records (slices) for a
// NoSQL table, identified by name or OCID. Optionally bounded by a time window and a per-page limit,
// walking pagination up to a safe cap.
package oracle_nosql_table_usage_list

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	ns "flomation.app/automate/executor/actions/oracle/nosql"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/nosql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI NoSQL: List Table Usage"
	Description  = "Report a NoSQL table's usage records (read/write throughput and storage per sampling period), identified by name or OCID. Optionally filter by a time window and limit; walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+table"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (needed when the table is given by name, not OCID)"},
	{Name: "table_ocid_or_name", Type: core.ConnectionTypeString, Label: "Table Name or OCID", Placeholder: "A table name within the compartment, or a table OCID", Required: true},
	{Name: "time_start", Type: core.ConnectionTypeString, Label: "Start Time", Placeholder: "RFC3339, e.g. 2026-07-01T00:00:00Z — omit for the most recent record (optional)"},
	{Name: "time_end", Type: core.ConnectionTypeString, Label: "End Time", Placeholder: "RFC3339, e.g. 2026-07-02T00:00:00Z (optional)"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max usage records per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "usage", Type: core.ConnectionTypeObject, Label: "Usage Records"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func parseRFC3339(name string, inputs []*core.Connection) (*common.SDKTime, error) {
	raw := strings.TrimSpace(ns.OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be an RFC3339 timestamp, e.g. 2026-07-01T00:00:00Z", strings.ReplaceAll(name, "_", " "))
	}
	return &common.SDKTime{Time: t}, nil
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := ns.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	tableNameOrID, err := ns.RequiredString("table_ocid_or_name", inputs)
	if err != nil {
		return ns.ErrorResult(err.Error()), nil
	}

	req := nosql.ListTableUsageRequest{TableNameOrId: &tableNameOrID}
	if c := auth.CompartmentOCID; c != "" {
		req.CompartmentId = &c
	}
	if req.TimeStart, err = parseRFC3339("time_start", inputs); err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	if req.TimeEnd, err = parseRFC3339("time_end", inputs); err != nil {
		return ns.ErrorResult(err.Error()), nil
	}
	if n, ok, err := ns.OptionalInt("limit", inputs); err != nil {
		return ns.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &n
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= ns.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListTableUsage(ns.Context(), req)
		if err != nil {
			return ns.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			u := &resp.Items[i]
			out = append(out, map[string]interface{}{
				"seconds_in_period":               ns.IntOrNil(u.SecondsInPeriod),
				"read_units":                      ns.IntOrNil(u.ReadUnits),
				"write_units":                     ns.IntOrNil(u.WriteUnits),
				"storage_in_gbs":                  ns.IntOrNil(u.StorageInGBs),
				"read_throttle_count":             ns.IntOrNil(u.ReadThrottleCount),
				"write_throttle_count":            ns.IntOrNil(u.WriteThrottleCount),
				"storage_throttle_count":          ns.IntOrNil(u.StorageThrottleCount),
				"max_shard_size_usage_in_percent": ns.IntOrNil(u.MaxShardSizeUsageInPercent),
				"time_started":                    ns.FormatTime(u.TimeStarted),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return ns.Result(fmt.Sprintf("Found %d usage record(s) for %q", len(out), tableNameOrID), map[string]interface{}{
		"usage": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
