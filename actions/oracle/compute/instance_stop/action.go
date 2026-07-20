// Package oracle_compute_instance_stop powers off a running OCI Compute instance.
// By default it requests a graceful shutdown (SOFTSTOP — signals the OS); enable
// "force" for an immediate power-off (STOP), which risks data loss.
package oracle_compute_instance_stop

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/oracle/compute"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Compute: Stop Instance"
	Description  = "Power off a running Oracle Cloud Compute instance. Defaults to a graceful OS shutdown; enable Force for an immediate power-off."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+stop"
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
	{Name: "force", Type: core.ConnectionTypeBoolean, Label: "Force (immediate power-off)", Placeholder: "Skip the graceful OS shutdown — risks data loss"},
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
	action := ocicore.InstanceActionActionSoftstop
	if compute.OptionalBool("force", inputs, false) {
		action = ocicore.InstanceActionActionStop
	}
	resp, err := client.InstanceAction(compute.Context(), ocicore.InstanceActionRequest{
		InstanceId: &id,
		Action:     action,
	})
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Stop requested for instance %s (now %s)", id, string(resp.LifecycleState)),
		"instance":        compute.SummariseInstance(&resp.Instance),
		"lifecycle_state": string(resp.LifecycleState),
		"success":         true,
	}, nil
}
