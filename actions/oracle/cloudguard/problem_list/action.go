// Package oracle_cloudguard_problem_list lists the security problems Cloud Guard has surfaced in a
// compartment, optionally filtered by lifecycle detail, lifecycle state, or risk level. Walks
// pagination up to a safe cap.
package oracle_cloudguard_problem_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	cg "flomation.app/automate/executor/actions/oracle/cloudguard"

	"github.com/oracle/oci-go-sdk/v65/cloudguard"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Cloud Guard: List Problems"
	Description  = "List the security problems Cloud Guard has surfaced in a compartment, optionally filtered by lifecycle detail, lifecycle state, or risk level. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
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
	{Name: "lifecycle_detail", Type: core.ConnectionTypeString, Label: "Lifecycle Detail", Placeholder: "Defaults to OPEN when unset", Options: []core.ConnectionOption{
		{Name: "Open", Value: "OPEN"}, {Name: "Resolved", Value: "RESOLVED"}, {Name: "Dismissed", Value: "DISMISSED"}, {Name: "Deleted", Value: "DELETED"},
	}},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Defaults to ACTIVE when unset", Options: []core.ConnectionOption{
		{Name: "Active", Value: "ACTIVE"}, {Name: "Inactive", Value: "INACTIVE"},
	}},
	{Name: "risk_level", Type: core.ConnectionTypeString, Label: "Risk Level", Placeholder: "Only problems at this risk level (optional)", Options: []core.ConnectionOption{
		{Name: "Critical", Value: "CRITICAL"}, {Name: "High", Value: "HIGH"}, {Name: "Medium", Value: "MEDIUM"}, {Name: "Low", Value: "LOW"}, {Name: "Minor", Value: "MINOR"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Items per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "problems", Type: core.ConnectionTypeObject, Label: "Problems"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := cg.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}
	req := cloudguard.ListProblemsRequest{CompartmentId: &compartment}
	if detail := cg.OptionalString("lifecycle_detail", inputs); detail != "" {
		req.LifecycleDetail = cloudguard.ListProblemsLifecycleDetailEnum(detail)
	}
	if state := cg.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = cloudguard.ListProblemsLifecycleStateEnum(state)
	}
	if risk := cg.OptionalString("risk_level", inputs); risk != "" {
		req.RiskLevel = &risk
	}
	if limit, ok, err := cg.OptionalInt("limit", inputs); err != nil {
		return cg.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= cg.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListProblems(cg.Context(), req)
		if err != nil {
			return cg.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, cg.SummariseProblemSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return cg.Result(fmt.Sprintf("Found %d problem(s)", len(out)), map[string]interface{}{
		"problems": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
