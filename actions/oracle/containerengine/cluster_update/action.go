// Package oracle_containerengine_cluster_update renames an OKE cluster and/or upgrades its
// Kubernetes control-plane version. This is asynchronous — it returns a work-request id;
// poll Get Work Request until the update completes.
package oracle_containerengine_cluster_update

import (
	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Update Cluster"
	Description  = "Rename an Oracle Cloud OKE cluster and/or upgrade its Kubernetes control-plane version. Asynchronous — returns a work-request id; poll Get Work Request until it completes."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the cluster picker)"},
	{Name: "cluster_ocid", Type: core.ConnectionTypeString, Label: "Cluster OCID", Placeholder: "ocid1.cluster.oc1..aaaa…", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "New Cluster Name", Placeholder: "Rename the cluster (optional)"},
	{Name: "kubernetes_version", Type: core.ConnectionTypeString, Label: "Kubernetes Version", Placeholder: "e.g. v1.30.1 — upgrade the control plane (optional; see Get Cluster Options)"},
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
	id, err := oke.RequiredString("cluster_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	details := okesdk.UpdateClusterDetails{}
	if name := oke.OptionalString("name", inputs); name != "" {
		details.Name = &name
	}
	if k8sVersion := oke.OptionalString("kubernetes_version", inputs); k8sVersion != "" {
		details.KubernetesVersion = &k8sVersion
	}
	resp, err := client.UpdateCluster(oke.Context(), okesdk.UpdateClusterRequest{ClusterId: &id, UpdateClusterDetails: details})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	return oke.AsyncResult("Updating cluster — poll Get Work Request until it completes", oke.Str(resp.OpcWorkRequestId)), nil
}
