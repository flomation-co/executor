// Package oracle_bastion_session_list lists the sessions on a bastion, optionally filtered by
// lifecycle state or exact display name. Walks pagination up to a safe cap.
package oracle_bastion_session_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	bas "flomation.app/automate/executor/actions/oracle/bastion"

	"github.com/oracle/oci-go-sdk/v65/bastion"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Bastion: List Sessions"
	Description  = "List the sessions on a bastion. Optionally filter by lifecycle state or exact display name and cap the page size. Walks pagination up to a safe limit."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+terminal"
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
	{Name: "bastion_ocid", Type: core.ConnectionTypeString, Label: "Bastion OCID", Placeholder: "ocid1.bastion.oc1..aaaa… — the bastion whose sessions to list", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only sessions with this exact name (optional)"},
	{Name: "session_lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Filter by session state (optional)", Options: []core.ConnectionOption{
		{Name: "Creating", Value: "CREATING"}, {Name: "Active", Value: "ACTIVE"}, {Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"}, {Name: "Failed", Value: "FAILED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max sessions per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "sessions", Type: core.ConnectionTypeObject, Label: "Sessions"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := bas.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	bastionID, err := bas.RequiredString("bastion_ocid", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}
	req := bastion.ListSessionsRequest{BastionId: &bastionID}
	if name := bas.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if state := bas.OptionalString("session_lifecycle_state", inputs); state != "" {
		req.SessionLifecycleState = bastion.ListSessionsSessionLifecycleStateEnum(state)
	}
	limit, err := bas.OptionalInt("limit", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}
	if limit != nil {
		req.Limit = limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= bas.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListSessions(bas.Context(), req)
		if err != nil {
			return bas.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, bas.SummariseSessionSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return bas.Result(fmt.Sprintf("Found %d session(s)", len(out)), map[string]interface{}{
		"sessions": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
