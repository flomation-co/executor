// Package oracle_waa_web_app_acceleration_create attaches a Web App Acceleration policy to a
// load-balancer backend, creating the WebAppAcceleration resource. Asynchronous: the resource
// comes back CREATING with a work-request id — poll Get Web App Acceleration until it is ACTIVE.
package oracle_waa_web_app_acceleration_create

import (
	core "flomation.app/automate/executor"
	wa "flomation.app/automate/executor/actions/oracle/waa"

	"github.com/oracle/oci-go-sdk/v65/waa"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Web Application Acceleration: Create Web App Acceleration"
	Description  = "Attach a Web App Acceleration policy to a load balancer, creating the acceleration. Returns a work-request id — poll Get Web App Acceleration until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+bolt"
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
	{Name: "policy_ocid", Type: core.ConnectionTypeString, Label: "Acceleration Policy OCID", Placeholder: "ocid1.webappaccelerationpolicy.oc1..aaaa…", Required: true},
	{Name: "load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Load Balancer OCID", Placeholder: "ocid1.loadbalancer.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the acceleration (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := wa.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return wa.ErrorResult(err.Error()), nil
	}
	policyID, err := wa.RequiredString("policy_ocid", inputs)
	if err != nil {
		return wa.ErrorResult(err.Error()), nil
	}
	lbID, err := wa.RequiredString("load_balancer_ocid", inputs)
	if err != nil {
		return wa.ErrorResult(err.Error()), nil
	}

	details := waa.CreateWebAppAccelerationLoadBalancerDetails{
		CompartmentId:              &compartment,
		WebAppAccelerationPolicyId: &policyID,
		LoadBalancerId:             &lbID,
	}
	if name := wa.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}

	resp, err := client.CreateWebAppAcceleration(wa.Context(), waa.CreateWebAppAccelerationRequest{CreateWebAppAccelerationDetails: details})
	if err != nil {
		return wa.ErrorResult(auth.OCIError(err)), nil
	}
	return wa.Result("Creating Web App Acceleration — poll Get Web App Acceleration until ACTIVE", map[string]interface{}{
		"work_request_id": wa.Str(resp.OpcWorkRequestId),
	}), nil
}
