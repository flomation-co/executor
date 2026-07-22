// Package oracle_containerengine_addon_get reads one add-on installed on an OKE cluster by
// name, including its version and lifecycle state.
package oracle_containerengine_addon_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Get Add-on"
	Description  = "Fetch a single add-on installed on an Oracle Cloud OKE cluster by name — its version, installed version, and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+cubes"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "cluster_ocid", Type: core.ConnectionTypeString, Label: "Cluster OCID", Placeholder: "ocid1.cluster.oc1..aaaa…", Required: true},
	{Name: "addon_name", Type: core.ConnectionTypeString, Label: "Add-on Name", Placeholder: "e.g. KubernetesDashboard", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "addon", Type: core.ConnectionTypeObject, Label: "Add-on"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Add-on Name"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := oke.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	clusterID, err := oke.RequiredString("cluster_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	addonName, err := oke.RequiredString("addon_name", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetAddon(oke.Context(), okesdk.GetAddonRequest{ClusterId: &clusterID, AddonName: &addonName})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	addon := oke.SummariseAddon(&resp.Addon)
	return oke.Result(fmt.Sprintf("Add-on %q is %s", addon["name"], addon["lifecycle_state"]), map[string]interface{}{
		"addon": addon, "name": addon["name"], "lifecycle_state": addon["lifecycle_state"],
	}), nil
}
