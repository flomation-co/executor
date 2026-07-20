// Package oracle_compute_instance_start powers on a stopped OCI Compute instance.
package oracle_compute_instance_start

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/oracle/compute"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Compute: Start Instance"
	Description  = "Power on a stopped Oracle Cloud Compute instance."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+play"
	Date         = "20/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "instance_ocid", Type: core.ConnectionTypeString, Label: "Instance OCID", Placeholder: "ocid1.instance.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance", Type: core.ConnectionTypeObject, Label: "Instance"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errRes := compute.PerInstanceClient(inputs)
	if errRes != nil {
		return errRes, nil
	}
	resp, err := client.InstanceAction(compute.Context(), ocicore.InstanceActionRequest{
		InstanceId: &id,
		Action:     ocicore.InstanceActionActionStart,
	})
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Start requested for instance %s (now %s)", id, string(resp.LifecycleState)),
		"instance":        compute.SummariseInstance(&resp.Instance),
		"lifecycle_state": string(resp.LifecycleState),
		"success":         true,
	}, nil
}
