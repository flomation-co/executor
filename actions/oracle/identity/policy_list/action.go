// Package oracle_identity_policy_list lists the IAM policies in a compartment (the tenancy).
package oracle_identity_policy_list

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: List Policies"
	Description  = "List the Oracle Cloud IAM policies in a compartment (the tenancy), optionally filtered by exact name. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+file-lines"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… (the caller's user, for signing)", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "the tenancy home region, e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (IAM policies live in the root)"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name Filter", Placeholder: "Only the policy with this exact name (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "policies", Type: core.ConnectionTypeObject, Label: "Policies"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := iam.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment := auth.CompartmentOrTenancy()
	req := identity.ListPoliciesRequest{CompartmentId: &compartment}
	if v := strings.TrimSpace(iam.OptionalString("name", inputs)); v != "" {
		req.Name = &v
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= iam.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListPolicies(iam.Context(), req)
		if err != nil {
			return iam.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, iam.SummarisePolicy(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return iam.Result(fmt.Sprintf("Found %d policy/policies", len(out)), map[string]interface{}{
		"policies": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
