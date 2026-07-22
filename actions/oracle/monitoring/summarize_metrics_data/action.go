// Package oracle_monitoring_summarize_metrics_data retrieves aggregated metric data: an MQL query
// evaluated over a namespace within an optional time range, returning one metric stream per
// dimension combination, each with its timestamp/value datapoints. Synchronous.
package oracle_monitoring_summarize_metrics_data

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	mon "flomation.app/automate/executor/actions/oracle/monitoring"

	ocicommon "github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/monitoring"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Monitoring: Summarize Metrics Data"
	Description  = "Retrieve aggregated metric data with an MQL query over a namespace. Returns one metric stream per dimension combination, each with timestamp/value datapoints. Time range defaults to the last 3 hours; supply RFC3339 start/end times to override."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+gauge"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Metric Namespace", Placeholder: "e.g. oci_computeagent", Required: true},
	{Name: "query", Type: core.ConnectionTypeText, Label: "MQL Query", Placeholder: "e.g. CpuUtilization[1m].mean()", Required: true},
	{Name: "start_time", Type: core.ConnectionTypeString, Label: "Start Time", Placeholder: "RFC3339, e.g. 2026-07-22T09:00:00Z (optional, defaults to 3h ago)"},
	{Name: "end_time", Type: core.ConnectionTypeString, Label: "End Time", Placeholder: "RFC3339, e.g. 2026-07-22T12:00:00Z (optional, defaults to now)"},
	{Name: "resolution", Type: core.ConnectionTypeString, Label: "Resolution", Placeholder: "e.g. 5m — aggregation window frequency (optional, default 1m)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "metric_data", Type: core.ConnectionTypeObject, Label: "Metric Data"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := mon.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	namespace, err := mon.RequiredString("namespace", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	query, err := mon.RequiredString("query", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}

	details := monitoring.SummarizeMetricsDataDetails{
		Namespace: &namespace,
		Query:     &query,
	}
	if v := mon.OptionalString("start_time", inputs); v != "" {
		parsed, perr := time.Parse(time.RFC3339, v)
		if perr != nil {
			return mon.ErrorResult("start time must be RFC3339, e.g. 2026-07-22T09:00:00Z"), nil
		}
		details.StartTime = &ocicommon.SDKTime{Time: parsed.UTC()}
	}
	if v := mon.OptionalString("end_time", inputs); v != "" {
		parsed, perr := time.Parse(time.RFC3339, v)
		if perr != nil {
			return mon.ErrorResult("end time must be RFC3339, e.g. 2026-07-22T12:00:00Z"), nil
		}
		details.EndTime = &ocicommon.SDKTime{Time: parsed.UTC()}
	}
	if v := mon.OptionalString("resolution", inputs); v != "" {
		details.Resolution = &v
	}

	resp, err := client.SummarizeMetricsData(mon.Context(), monitoring.SummarizeMetricsDataRequest{
		CompartmentId:               &compartment,
		SummarizeMetricsDataDetails: details,
	})
	if err != nil {
		return mon.ErrorResult(auth.OCIError(err)), nil
	}

	out := make([]map[string]interface{}, 0, len(resp.Items))
	for i := range resp.Items {
		md := &resp.Items[i]
		points := make([]map[string]interface{}, 0, len(md.AggregatedDatapoints))
		for j := range md.AggregatedDatapoints {
			dp := md.AggregatedDatapoints[j]
			var value interface{}
			if dp.Value != nil {
				value = *dp.Value
			}
			points = append(points, map[string]interface{}{
				"timestamp": mon.FormatTime(dp.Timestamp),
				"value":     value,
			})
		}
		out = append(out, map[string]interface{}{
			"name":           mon.Str(md.Name),
			"namespace":      mon.Str(md.Namespace),
			"compartment_id": mon.Str(md.CompartmentId),
			"resource_group": mon.Str(md.ResourceGroup),
			"resolution":     mon.Str(md.Resolution),
			"dimensions":     md.Dimensions,
			"datapoints":     points,
		})
	}

	return mon.Result(fmt.Sprintf("Returned %d metric stream(s)", len(out)), map[string]interface{}{
		"metric_data": out, "count": fmt.Sprintf("%d", len(out)),
	}), nil
}
