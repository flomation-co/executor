// Package oracle_compute_instance_terminate permanently deletes an OCI Compute
// instance. The boot volume is PRESERVED by default (matching the OCI console),
// so terminate can't silently destroy data; enable "delete boot volume" to
// remove it too.
package oracle_compute_instance_terminate

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/oracle/compute"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Compute: Terminate Instance"
	Description  = "Permanently delete an Oracle Cloud Compute instance. The boot volume is preserved by default (matching the OCI console); enable Delete Boot Volume to remove it too."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+trash"
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
	{Name: "delete_boot_volume", Type: core.ConnectionTypeBoolean, Label: "Delete Boot Volume", Placeholder: "Also permanently delete the boot volume (default: preserved, matching the OCI console)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance_ocid", Type: core.ConnectionTypeString, Label: "Instance OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errRes := compute.PerInstanceClient(inputs)
	if errRes != nil {
		return errRes, nil
	}
	// Boot volume is PRESERVED by default (matching the OCI console); deletion is
	// an explicit opt-in so terminate can't silently destroy data.
	deleteBootVol := compute.OptionalBool("delete_boot_volume", inputs, false)
	preserve := !deleteBootVol
	_, err := client.TerminateInstance(compute.Context(), ocicore.TerminateInstanceRequest{
		InstanceId:         &id,
		PreserveBootVolume: &preserve,
	})
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	msg := fmt.Sprintf("Termination requested for instance %s", id)
	if preserve {
		msg += " (boot volume preserved)"
	} else {
		msg += " (boot volume deleted)"
	}
	return map[string]interface{}{
		"tool_result":   msg,
		"instance_ocid": id,
		"success":       true,
	}, nil
}
