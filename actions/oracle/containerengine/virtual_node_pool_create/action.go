// Package oracle_containerengine_virtual_node_pool_create adds a virtual node pool to an OKE
// cluster — serverless pods scheduled by OCI rather than nodes you run. Asynchronous —
// returns a work-request id; poll Get Work Request until it completes, then Get Virtual Node Pool.
package oracle_containerengine_virtual_node_pool_create

import (
	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Create Virtual Node Pool"
	Description  = "Add a virtual node pool to an Oracle Cloud OKE cluster — serverless pods OCI schedules for you, placed in an availability domain + subnet with a pod subnet and pod shape. Asynchronous — returns a work-request id; poll Get Work Request, then Get Virtual Node Pool."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "cluster_ocid", Type: core.ConnectionTypeString, Label: "Cluster OCID", Placeholder: "ocid1.cluster.oc1..aaaa… the pool belongs to", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Virtual Node Pool Name", Placeholder: "A name for the virtual node pool", Required: true},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Vgqo:UK-LONDON-1-AD-1 — where the virtual nodes land", Required: true},
	{Name: "subnet_ocid", Type: core.ConnectionTypeString, Label: "Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… for the virtual nodes", Required: true},
	{Name: "pod_subnet_ocid", Type: core.ConnectionTypeString, Label: "Pod Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… (regional) where pods' VNICs land", Required: true},
	{Name: "pod_shape", Type: core.ConnectionTypeString, Label: "Pod Shape", Placeholder: "e.g. Pod.Standard.E4.Flex — the shape of the pods", Required: true},
	{Name: "size", Type: core.ConnectionTypeString, Label: "Virtual Node Count", Placeholder: "How many virtual nodes to launch (default 1)"},
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
	name, err := oke.RequiredString("display_name", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	adom, err := oke.RequiredString("availability_domain", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	subnet, err := oke.RequiredString("subnet_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	podSubnet, err := oke.RequiredString("pod_subnet_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	podShape, err := oke.RequiredString("pod_shape", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	size, ok, err := oke.OptionalInt("size", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	if !ok {
		size = 1
	}
	details := okesdk.CreateVirtualNodePoolDetails{
		CompartmentId: &compartment,
		ClusterId:     &clusterID,
		DisplayName:   &name,
		Size:          &size,
		PlacementConfigurations: []okesdk.PlacementConfiguration{
			{AvailabilityDomain: &adom, SubnetId: &subnet},
		},
		PodConfiguration: &okesdk.PodConfiguration{
			SubnetId: &podSubnet,
			Shape:    &podShape,
			NsgIds:   nil,
		},
	}
	resp, err := client.CreateVirtualNodePool(oke.Context(), okesdk.CreateVirtualNodePoolRequest{CreateVirtualNodePoolDetails: details})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	return oke.AsyncResult("Creating virtual node pool "+name+" — poll Get Work Request until it completes", oke.Str(resp.OpcWorkRequestId)), nil
}
