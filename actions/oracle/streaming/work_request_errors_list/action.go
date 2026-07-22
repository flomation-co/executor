// Package oracle_streaming_work_request_errors_list lists the errors recorded against one
// Streaming work request — the machine-usable code, human-readable message and timestamp of each
// failure that occurred while the asynchronous operation ran. Walks pagination up to a safe cap.
package oracle_streaming_work_request_errors_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	str "flomation.app/automate/executor/actions/oracle/streaming"

	"github.com/oracle/oci-go-sdk/v65/streaming"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Streaming: List Work Request Errors"
	Description  = "List the errors recorded against a Streaming work request — each error's code, message and timestamp."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "work_request_ocid", Type: core.ConnectionTypeString, Label: "Work Request OCID", Placeholder: "ocid1.streamingworkrequest.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "errors", Type: core.ConnectionTypeObject, Label: "Errors"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := str.AdminClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	workRequestID, err := str.RequiredString("work_request_ocid", inputs)
	if err != nil {
		return str.ErrorResult(err.Error()), nil
	}
	req := streaming.ListWorkRequestErrorsRequest{WorkRequestId: &workRequestID}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= str.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListWorkRequestErrors(str.Context(), req)
		if err != nil {
			return str.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			e := &resp.Items[i]
			out = append(out, map[string]interface{}{
				"code":      str.Str(e.Code),
				"message":   str.Str(e.Message),
				"timestamp": str.FormatTime(e.Timestamp),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return str.Result(fmt.Sprintf("Found %d work request error(s)", len(out)), map[string]interface{}{
		"errors": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
