// Package oracle_apigateway_deployment_change_compartment moves an API Gateway deployment from one
// compartment to another. The deployment keeps its OCID; only its compartment placement (for access
// control and billing) changes.
package oracle_apigateway_deployment_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	agw "flomation.app/automate/executor/actions/oracle/apigateway"

	"github.com/oracle/oci-go-sdk/v65/apigateway"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI API Gateway: Change Deployment Compartment"
	Description  = "Move an API Gateway deployment into a different compartment — the deployment keeps its OCID, only its compartment placement changes."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+route"
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
	{Name: "deployment_ocid", Type: core.ConnectionTypeString, Label: "Deployment OCID", Placeholder: "ocid1.apideployment.oc1..aaaa… (the deployment to move)", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where to move the deployment)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Deployment OCID"},
	{Name: "destination_compartment_id", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := agw.DeploymentClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	deploymentID, err := agw.RequiredString("deployment_ocid", inputs)
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}
	destination, err := agw.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return agw.ErrorResult(err.Error()), nil
	}

	_, err = client.ChangeDeploymentCompartment(agw.Context(), apigateway.ChangeDeploymentCompartmentRequest{
		DeploymentId: &deploymentID,
		ChangeDeploymentCompartmentDetails: apigateway.ChangeDeploymentCompartmentDetails{
			CompartmentId: &destination,
		},
	})
	if err != nil {
		return agw.ErrorResult(auth.OCIError(err)), nil
	}

	return agw.Result(fmt.Sprintf("Moved deployment %s into compartment %s", deploymentID, destination), map[string]interface{}{
		"id":                         deploymentID,
		"destination_compartment_id": destination,
	}), nil
}
