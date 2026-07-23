// Package oracle_dataflow_private_endpoint_create creates a Data Flow private endpoint — the network
// attachment that lets Spark applications reach private resources (databases, services) inside a VCN
// subnet by their DNS names. Asynchronous: the endpoint comes back CREATING with a work-request id;
// poll the Get Private Endpoint action until it is ACTIVE before use.
package oracle_dataflow_private_endpoint_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	df "flomation.app/automate/executor/actions/oracle/dataflow"

	"github.com/oracle/oci-go-sdk/v65/dataflow"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Flow: Create Private Endpoint"
	Description  = "Create a Data Flow private endpoint into a VCN subnet so Spark can reach private resources by their DNS zone names. Returns a work-request id — poll Get Private Endpoint until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+diagram-project"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "subnet_ocid", Type: core.ConnectionTypeString, Label: "Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… — the VCN subnet to attach into", Required: true},
	{Name: "dns_zones", Type: core.ConnectionTypeString, Label: "DNS Zones (comma-separated)", Placeholder: "e.g. app.examplecorp.com, db.examplecorp.com", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the private endpoint (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := df.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	subnet, err := df.RequiredString("subnet_ocid", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	zonesRaw, err := df.RequiredString("dns_zones", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	var zones []string
	for _, z := range strings.Split(zonesRaw, ",") {
		if z = strings.TrimSpace(z); z != "" {
			zones = append(zones, z)
		}
	}
	if len(zones) == 0 {
		return df.ErrorResult("dns zones is required — provide at least one DNS zone name"), nil
	}

	details := dataflow.CreatePrivateEndpointDetails{
		CompartmentId: &compartment,
		SubnetId:      &subnet,
		DnsZones:      zones,
	}
	if name := strings.TrimSpace(df.OptionalString("display_name", inputs)); name != "" {
		details.DisplayName = &name
	}

	resp, err := client.CreatePrivateEndpoint(df.Context(), dataflow.CreatePrivateEndpointRequest{CreatePrivateEndpointDetails: details})
	if err != nil {
		return df.ErrorResult(auth.OCIError(err)), nil
	}
	return df.Result(
		fmt.Sprintf("Creating private endpoint into subnet %s — poll Get Private Endpoint until ACTIVE", subnet),
		map[string]interface{}{
			"work_request_id": df.Str(resp.OpcWorkRequestId),
		},
	), nil
}
