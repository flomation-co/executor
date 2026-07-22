// Package oracle_monitoring_post_metric_data publishes a custom metric datapoint. This is the one
// Monitoring operation that uses the telemetry-INGESTION endpoint rather than the query endpoint —
// the shared IngestionClient handles that host swap.
package oracle_monitoring_post_metric_data

import (
	"fmt"
	"strconv"
	"time"

	core "flomation.app/automate/executor"
	mon "flomation.app/automate/executor/actions/oracle/monitoring"

	ocicommon "github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/monitoring"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Monitoring: Post Metric Data"
	Description  = "Publish a custom metric datapoint to a namespace with dimensions. Uses the telemetry-ingestion endpoint automatically. Timestamp defaults to now; supply an RFC3339 time to override."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where the metric is posted)", Required: true},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "e.g. my_app (custom namespace)", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Metric Name", Placeholder: "e.g. orders_processed", Required: true},
	{Name: "value", Type: core.ConnectionTypeString, Label: "Value", Placeholder: "The datapoint value, e.g. 42.5", Required: true},
	{Name: "dimensions", Type: core.ConnectionTypeString, Label: "Dimensions (JSON)", Placeholder: "{\"resourceId\":\"ocid1...\"} — at least one required", Required: true},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Timestamp", Placeholder: "RFC3339 (optional, defaults to now)"},
	{Name: "resource_group", Type: core.ConnectionTypeString, Label: "Resource Group", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "failed_count", Type: core.ConnectionTypeInteger, Label: "Failed datapoints"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	// IngestionClient points at telemetry-ingestion.<region> — required for PostMetricData.
	auth, client, errResult := mon.IngestionClient(inputs)
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
	name, err := mon.RequiredString("name", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	valueRaw, err := mon.RequiredString("value", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	value, convErr := strconv.ParseFloat(valueRaw, 64)
	if convErr != nil {
		return mon.ErrorResult("value must be a number"), nil
	}
	dims, err := mon.DimensionsMap("dimensions", inputs)
	if err != nil {
		return mon.ErrorResult(err.Error()), nil
	}
	if len(dims) == 0 {
		return mon.ErrorResult("at least one dimension is required"), nil
	}

	ts := time.Now().UTC()
	if v := mon.OptionalString("timestamp", inputs); v != "" {
		parsed, perr := time.Parse(time.RFC3339, v)
		if perr != nil {
			return mon.ErrorResult("timestamp must be RFC3339, e.g. 2026-07-22T12:00:00Z"), nil
		}
		ts = parsed.UTC()
	}

	datum := monitoring.MetricDataDetails{
		Namespace:     &namespace,
		CompartmentId: &compartment,
		Name:          &name,
		Dimensions:    dims,
		Datapoints:    []monitoring.Datapoint{{Timestamp: &ocicommon.SDKTime{Time: ts}, Value: &value}},
	}
	if rg := mon.OptionalString("resource_group", inputs); rg != "" {
		datum.ResourceGroup = &rg
	}

	resp, err := client.PostMetricData(mon.Context(), monitoring.PostMetricDataRequest{
		PostMetricDataDetails: monitoring.PostMetricDataDetails{MetricData: []monitoring.MetricDataDetails{datum}},
	})
	if err != nil {
		return mon.ErrorResult(auth.OCIError(err)), nil
	}
	failed := 0
	if resp.FailedMetricsCount != nil {
		failed = *resp.FailedMetricsCount
	}
	return mon.Result(fmt.Sprintf("Posted metric %q to namespace %q (%d failed)", name, namespace, failed), map[string]interface{}{
		"failed_count": failed,
	}), nil
}
