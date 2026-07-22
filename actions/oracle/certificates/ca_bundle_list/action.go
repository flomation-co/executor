// Package oracle_certificates_ca_bundle_list lists the CA bundles in a compartment, optionally
// filtered by exact name or lifecycle state. Walks pagination up to a safe cap.
package oracle_certificates_ca_bundle_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: List CA Bundles"
	Description  = "List the CA bundles in a compartment. Optionally filter by exact name or lifecycle state, and cap the page size. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+id-badge"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name Filter", Placeholder: "Only CA bundles with this exact name (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Filter by lifecycle state (optional)", Options: []core.ConnectionOption{
		{Name: "Creating", Value: "CREATING"},
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Updating", Value: "UPDATING"},
		{Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"},
		{Name: "Failed", Value: "FAILED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max items per page fetch (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "ca_bundles", Type: core.ConnectionTypeObject, Label: "CA Bundles"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := certs.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}
	req := certificatesmanagement.ListCaBundlesRequest{CompartmentId: &compartment}
	if name := certs.OptionalString("name", inputs); name != "" {
		req.Name = &name
	}
	if state := certs.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = certificatesmanagement.ListCaBundlesLifecycleStateEnum(state)
	}
	if limit, ok, err := certs.OptionalInt("limit", inputs); err != nil {
		return certs.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= certs.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListCaBundles(certs.Context(), req)
		if err != nil {
			return certs.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, certs.SummariseCaBundleSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return certs.Result(fmt.Sprintf("Found %d CA bundle(s)", len(out)), map[string]interface{}{
		"ca_bundles": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
