// Package oracle_cloudguard_problem_update_status changes the workflow status of a Cloud Guard
// problem — mark it OPEN, RESOLVED or DISMISSED — and returns the updated problem.
package oracle_cloudguard_problem_update_status

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	cg "flomation.app/automate/executor/actions/oracle/cloudguard"

	"github.com/oracle/oci-go-sdk/v65/cloudguard"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Cloud Guard: Update Problem Status"
	Description  = "Change a Cloud Guard problem's workflow status — mark it OPEN, RESOLVED or DISMISSED."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "problem_ocid", Type: core.ConnectionTypeString, Label: "Problem OCID", Placeholder: "ocid1.cloudguardproblem.oc1..aaaa… — the problem to update", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "The new workflow status", Required: true, Options: []core.ConnectionOption{
		{Name: "Open", Value: "OPEN"},
		{Name: "Resolved", Value: "RESOLVED"},
		{Name: "Dismissed", Value: "DISMISSED"},
	}},
	{Name: "comment", Type: core.ConnectionTypeText, Label: "Comment", Placeholder: "Optional note recorded with the status change"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "problem", Type: core.ConnectionTypeObject, Label: "Problem"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Problem OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := cg.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	problemID, err := cg.RequiredString("problem_ocid", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}
	statusRaw, err := cg.RequiredString("status", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}
	status, ok := cloudguard.GetMappingProblemLifecycleDetailEnum(statusRaw)
	if !ok {
		return cg.ErrorResult(fmt.Sprintf("status %q is not valid — expected one of OPEN, RESOLVED, DISMISSED", statusRaw)), nil
	}

	details := cloudguard.UpdateProblemStatusDetails{Status: status}
	if v := strings.TrimSpace(cg.OptionalString("comment", inputs)); v != "" {
		details.Comment = &v
	}

	resp, err := client.UpdateProblemStatus(cg.Context(), cloudguard.UpdateProblemStatusRequest{
		ProblemId:                  &problemID,
		UpdateProblemStatusDetails: details,
	})
	if err != nil {
		return cg.ErrorResult(auth.OCIError(err)), nil
	}
	problem := cg.SummariseProblem(&resp.Problem)
	return cg.Result(fmt.Sprintf("Updated problem %s status to %s", cg.Str(resp.Problem.Id), string(status)), map[string]interface{}{
		"problem": problem, "id": problem["id"],
	}), nil
}
