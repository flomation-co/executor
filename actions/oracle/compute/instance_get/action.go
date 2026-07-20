// Package oracle_compute_instance_get fetches a single OCI Compute instance by
// its OCID, returning its full lifecycle state, shape, availability domain,
// region and tags.
package oracle_compute_instance_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/oracle/compute"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Compute: Get Instance"
	Description  = "Fetch a single Oracle Cloud Compute instance by its OCID — lifecycle state, shape, availability domain, region and tags."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+magnifying-glass"
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
	resp, err := client.GetInstance(compute.Context(), ocicore.GetInstanceRequest{InstanceId: &id})
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	inst := compute.SummariseInstance(&resp.Instance)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Instance %q is %s", compute.Str(resp.DisplayName), string(resp.LifecycleState)),
		"instance":        inst,
		"lifecycle_state": string(resp.LifecycleState),
		"success":         true,
	}, nil
}
