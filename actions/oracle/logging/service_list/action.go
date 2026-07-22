// Package oracle_logging_service_list lists every OCI service that supports (is integrated with)
// public logging — a tenancy-wide catalogue, so no compartment is needed. Each entry carries the
// service's user-friendly name, service-principal name, endpoint and the resource types it logs.
package oracle_logging_service_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	lg "flomation.app/automate/executor/actions/oracle/logging"

	"github.com/oracle/oci-go-sdk/v65/logging"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Logging: List Services"
	Description  = "List every OCI service that is integrated with public logging. Tenancy-wide, so no compartment is required."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+file-lines"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "services", Type: core.ConnectionTypeObject, Label: "Services"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := lg.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}

	resp, err := client.ListServices(lg.Context(), logging.ListServicesRequest{})
	if err != nil {
		return lg.ErrorResult(auth.OCIError(err)), nil
	}

	out := make([]map[string]interface{}, 0, len(resp.Items))
	for i := range resp.Items {
		s := resp.Items[i]
		var resourceTypes []string
		for _, rt := range s.ResourceTypes {
			resourceTypes = append(resourceTypes, lg.Str(rt.Name))
		}
		out = append(out, map[string]interface{}{
			"id":                     lg.Str(s.Id),
			"name":                   lg.Str(s.Name),
			"service_principal_name": lg.Str(s.ServicePrincipalName),
			"endpoint":               lg.Str(s.Endpoint),
			"tenant_id":              lg.Str(s.TenantId),
			"namespace":              lg.Str(s.Namespace),
			"resource_types":         resourceTypes,
		})
	}

	return lg.Result(fmt.Sprintf("Found %d logging-integrated service(s)", len(out)), map[string]interface{}{
		"services": out,
		"count":    fmt.Sprintf("%d", len(out)),
	}), nil
}
