// Package oracle_containerengine_kubeconfig_create generates a kubeconfig for an OKE cluster
// and returns its YAML content — feed it to kubectl or a downstream step to talk to the
// cluster's Kubernetes API.
package oracle_containerengine_kubeconfig_create

import (
	"io"
	"strings"

	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Create Kubeconfig"
	Description  = "Generate a kubeconfig for an Oracle Cloud OKE cluster and return its YAML content — hand it to kubectl or a downstream step to talk to the cluster. Defaults to the public API endpoint and a token that lasts the maximum allowed."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+cubes"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the cluster picker)"},
	{Name: "cluster_ocid", Type: core.ConnectionTypeString, Label: "Cluster OCID", Placeholder: "ocid1.cluster.oc1..aaaa… to generate the kubeconfig for", Required: true},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Endpoint", Placeholder: "Which API endpoint the kubeconfig targets — PUBLIC_ENDPOINT (default), PRIVATE_ENDPOINT or VCN_HOSTNAME", Options: []core.ConnectionOption{
		{Name: "Public endpoint", Value: "PUBLIC_ENDPOINT"},
		{Name: "Private endpoint", Value: "PRIVATE_ENDPOINT"},
		{Name: "VCN hostname", Value: "VCN_HOSTNAME"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "kubeconfig", Type: core.ConnectionTypeText, Label: "Kubeconfig (YAML)"},
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
	tokenVersion := "2.0.0"
	details := okesdk.CreateClusterKubeconfigContentDetails{TokenVersion: &tokenVersion}
	switch strings.ToUpper(strings.TrimSpace(oke.OptionalString("endpoint", inputs))) {
	case "PRIVATE_ENDPOINT":
		details.Endpoint = okesdk.CreateClusterKubeconfigContentDetailsEndpointPrivateEndpoint
	case "VCN_HOSTNAME":
		details.Endpoint = okesdk.CreateClusterKubeconfigContentDetailsEndpointVcnHostname
	case "", "PUBLIC_ENDPOINT":
		details.Endpoint = okesdk.CreateClusterKubeconfigContentDetailsEndpointPublicEndpoint
	default:
		return oke.ErrorResult("endpoint must be PUBLIC_ENDPOINT, PRIVATE_ENDPOINT or VCN_HOSTNAME"), nil
	}
	resp, err := client.CreateKubeconfig(oke.Context(), okesdk.CreateKubeconfigRequest{
		ClusterId:                             &id,
		CreateClusterKubeconfigContentDetails: details,
	})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	kubeconfig := ""
	if resp.Content != nil {
		defer resp.Content.Close()
		b, rerr := io.ReadAll(resp.Content)
		if rerr != nil {
			return oke.ErrorResult("kubeconfig generated but its content could not be read: " + rerr.Error()), nil
		}
		kubeconfig = string(b)
	}
	return oke.Result("Generated kubeconfig — capture the YAML", map[string]interface{}{
		"kubeconfig": kubeconfig,
	}), nil
}
