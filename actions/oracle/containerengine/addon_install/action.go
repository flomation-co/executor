// Package oracle_containerengine_addon_install installs (enables) a managed add-on on an OKE
// cluster, e.g. CertManager or KubernetesDashboard. Asynchronous — it returns a work-request
// id; poll Get Work Request until the installation completes.
package oracle_containerengine_addon_install

import (
	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Install Add-on"
	Description  = "Install (enable) a managed add-on such as CertManager or KubernetesDashboard on an Oracle Cloud OKE cluster. Asynchronous — returns a work-request id; poll Get Work Request until it completes."
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
	{Name: "addon_name", Type: core.ConnectionTypeString, Label: "Add-on Name", Placeholder: "e.g. CertManager or KubernetesDashboard", Required: true},
	{Name: "version", Type: core.ConnectionTypeString, Label: "Add-on Version", Placeholder: "Specific add-on version to install (optional — defaults to latest)"},
	{Name: "override_existing", Type: core.ConnectionTypeBoolean, Label: "Override Existing", Placeholder: "Override an existing installation of this add-on (optional, default false)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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
	d := okesdk.InstallAddonDetails{AddonName: &addonName}
	if v := oke.OptionalString("version", inputs); v != "" {
		d.Version = &v
	}
	if oke.BoolWasSet("override_existing", inputs) {
		b := oke.OptionalBool("override_existing", inputs, false)
		d.IsOverrideExisting = &b
	}
	resp, err := client.InstallAddon(oke.Context(), okesdk.InstallAddonRequest{ClusterId: &clusterID, InstallAddonDetails: d})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	return oke.AsyncResult("Installing add-on — poll Get Work Request until it completes", oke.Str(resp.OpcWorkRequestId)), nil
}
