// Package oracle_compute_instance_tag_set replaces the freeform tags on an OCI
// Compute instance — the focused "set tags" counterpart to Update Instance, for
// tag-only flows (cost allocation, ownership, environment labelling).
package oracle_compute_instance_tag_set

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/oracle/compute"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Compute: Set Instance Tags"
	Description  = "Replace the freeform tags on an Oracle Cloud Compute instance with a JSON object of string key/values. Replaces all freeform tags on the instance."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+tag"
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
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod","owner":"ops"}`, Required: true},
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
	tags, err := compute.FreeformTags("tags", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	// A blank input (nil) is "missing"; an explicit empty object ({}) is a valid
	// clear-all-tags request, matching instance_update's replace semantics.
	if tags == nil {
		return compute.ErrorResult("tags is required — supply a JSON object of string key/values, e.g. {\"env\":\"prod\"} (or {} to clear all tags)"), nil
	}

	resp, err := client.UpdateInstance(compute.Context(), ocicore.UpdateInstanceRequest{
		InstanceId:            &id,
		UpdateInstanceDetails: ocicore.UpdateInstanceDetails{FreeformTags: tags},
	})
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	msg := fmt.Sprintf("Set %d tag(s) on instance %s", len(tags), id)
	if len(tags) == 0 {
		msg = fmt.Sprintf("Cleared all tags on instance %s", id)
	}
	return map[string]interface{}{
		"tool_result":     msg,
		"instance":        compute.SummariseInstance(&resp.Instance),
		"lifecycle_state": string(resp.LifecycleState),
		"success":         true,
	}, nil
}
