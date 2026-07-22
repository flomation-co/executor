// Package oracle_streaming_stream_create creates a stream — the partitioned, append-only log
// that producers publish to and consumers read from. Provide EITHER a compartment (the stream
// lands in a default stream pool there) OR an explicit stream pool. Asynchronous: the stream
// comes back CREATING; poll the Get Stream action until it is ACTIVE before publishing.
package oracle_streaming_stream_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: Create Stream"
	Description  = "Create a Kafka-compatible stream. Give it a name and a partition count, and either a compartment (uses that compartment's default stream pool) or a specific stream pool. Returns the stream in a CREATING state — poll Get Stream until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+tower-broadcast"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (uses its default stream pool)"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Stream Name", Placeholder: "e.g. orders-events", Required: true},
	{Name: "partitions", Type: core.ConnectionTypeString, Label: "Partitions", Placeholder: "Number of partitions, e.g. 1", Required: true},
	{Name: "retention_hours", Type: core.ConnectionTypeString, Label: "Retention (hours)", Placeholder: "How long to keep messages, 24–168 (default 24)"},
	{Name: "stream_pool_id", Type: core.ConnectionTypeString, Label: "Stream Pool OCID", Placeholder: "ocid1.streampool.oc1..aaaa… (instead of a compartment)"},
	{Name: "freeform_tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: "{\"env\":\"prod\"} (optional)"},
	{Name: "defined_tags", Type: core.ConnectionTypeString, Label: "Defined Tags (JSON)", Placeholder: "{\"Ops\":{\"env\":\"prod\"}} (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "stream", Type: core.ConnectionTypeObject, Label: "Stream"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Stream OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "messages_endpoint", Type: core.ConnectionTypeString, Label: "Messages Endpoint"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := str.AdminClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	name, err := str.RequiredString("name", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	partitions, err := str.RequiredInt("partitions", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}

	details := streaming.CreateStreamDetails{Name: &name, Partitions: &partitions}
	// The service requires EXACTLY ONE of compartment or stream pool. Prefer the explicit pool;
	// otherwise place the stream (and a default pool) in the compartment.
	poolID := str.OptionalString("stream_pool_id", inputs)
	compartmentID := str.OptionalString("compartment_ocid", inputs)
	switch {
	case poolID != "":
		details.StreamPoolId = &poolID
	case compartmentID != "":
		details.CompartmentId = &compartmentID
	default:
		return str.ErrorResult("provide either a compartment OCID or a stream pool OCID"), nil
	}
	if n, ok, err := str.OptionalInt("retention_hours", inputs); err != nil {
		return str.ErrorResult(err.Error()), nil
	} else if ok {
		details.RetentionInHours = &n
	}
	if tags, err := str.FreeformTags("freeform_tags", inputs); err != nil {
		return str.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}
	if tags, err := str.DefinedTags("defined_tags", inputs); err != nil {
		return str.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.DefinedTags = tags
	}

	resp, err := client.CreateStream(str.Context(), streaming.CreateStreamRequest{CreateStreamDetails: details})
	if err != nil {
		return str.ErrorResult(auth.OCIError(err)), nil
	}
	stream := str.SummariseStream(&resp.Stream)
	return str.Result(fmt.Sprintf("Creating stream %q — now %s", stream["name"], stream["lifecycle_state"]), map[string]interface{}{
		"stream":            stream,
		"id":                stream["id"],
		"lifecycle_state":   stream["lifecycle_state"],
		"messages_endpoint": stream["messages_endpoint"],
		"work_request_id":   str.Str(resp.OpcWorkRequestId),
	}), nil
}
