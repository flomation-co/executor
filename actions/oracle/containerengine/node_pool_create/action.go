// Package oracle_containerengine_node_pool_create adds a managed worker node pool to an OKE
// cluster. Asynchronous — returns a work-request id; poll Get Work Request until the nodes
// are provisioning, then Get Node Pool for the node list.
package oracle_containerengine_node_pool_create

import (
	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Create Node Pool"
	Description  = "Add a managed worker node pool to an Oracle Cloud OKE cluster — pick the compute shape, node image, count and where the nodes land (availability domain + subnet). Asynchronous — returns a work-request id; poll Get Work Request, then Get Node Pool."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "cluster_ocid", Type: core.ConnectionTypeString, Label: "Cluster OCID", Placeholder: "ocid1.cluster.oc1..aaaa… the pool belongs to", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Node Pool Name", Placeholder: "A name for the node pool", Required: true},
	{Name: "kubernetes_version", Type: core.ConnectionTypeString, Label: "Kubernetes Version", Placeholder: "e.g. v1.30.1 (must match / trail the cluster)", Required: true},
	{Name: "node_shape", Type: core.ConnectionTypeString, Label: "Node Shape", Placeholder: "e.g. VM.Standard.E4.Flex (see Get Node Pool Options)", Required: true},
	{Name: "node_image_ocid", Type: core.ConnectionTypeString, Label: "Node Image OCID", Placeholder: "ocid1.image.oc1..aaaa… — an OKE worker image", Required: true},
	{Name: "size", Type: core.ConnectionTypeString, Label: "Node Count", Placeholder: "How many worker nodes to launch", Required: true},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Vgqo:UK-LONDON-1-AD-1 — where the nodes land", Required: true},
	{Name: "subnet_ocid", Type: core.ConnectionTypeString, Label: "Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… for the worker nodes", Required: true},
	{Name: "ssh_public_key", Type: core.ConnectionTypeText, Label: "SSH Public Key", Placeholder: "Public key to allow SSH to the nodes (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	clusterID, err := oke.RequiredString("cluster_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	name, err := oke.RequiredString("name", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	k8sVersion, err := oke.RequiredString("kubernetes_version", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	shape, err := oke.RequiredString("node_shape", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	imageID, err := oke.RequiredString("node_image_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	size, err := oke.RequiredInt("size", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	adom, err := oke.RequiredString("availability_domain", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	subnetID, err := oke.RequiredString("subnet_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	details := okesdk.CreateNodePoolDetails{
		CompartmentId:     &compartment,
		ClusterId:         &clusterID,
		Name:              &name,
		KubernetesVersion: &k8sVersion,
		NodeShape:         &shape,
		NodeSourceDetails: okesdk.NodeSourceViaImageDetails{ImageId: &imageID},
		NodeConfigDetails: &okesdk.CreateNodePoolNodeConfigDetails{
			Size: &size,
			PlacementConfigs: []okesdk.NodePoolPlacementConfigDetails{
				{AvailabilityDomain: &adom, SubnetId: &subnetID},
			},
		},
	}
	if ssh := oke.OptionalString("ssh_public_key", inputs); ssh != "" {
		details.SshPublicKey = &ssh
	}
	if tags, err := oke.FreeformTags("tags", inputs); err != nil {
		return oke.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	resp, err := client.CreateNodePool(oke.Context(), okesdk.CreateNodePoolRequest{CreateNodePoolDetails: details})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	return oke.AsyncResult("Creating node pool "+name+" — poll Get Work Request until it completes", oke.Str(resp.OpcWorkRequestId)), nil
}
