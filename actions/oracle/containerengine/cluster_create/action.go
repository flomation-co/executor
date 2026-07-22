// Package oracle_containerengine_cluster_create provisions a new OKE Kubernetes cluster.
// This is asynchronous — it returns a work-request id; poll Get Work Request until the
// cluster is created, then Get Cluster for its endpoints.
package oracle_containerengine_cluster_create

import (
	"strings"

	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Create Cluster"
	Description  = "Provision a new Oracle Cloud OKE Kubernetes cluster in a VCN. Asynchronous — returns a work-request id; poll Get Work Request until it completes, then Get Cluster for the API endpoints."
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Cluster Name", Placeholder: "A name for the cluster", Required: true},
	{Name: "vcn_ocid", Type: core.ConnectionTypeString, Label: "VCN OCID", Placeholder: "ocid1.vcn.oc1..aaaa… the cluster runs in", Required: true},
	{Name: "kubernetes_version", Type: core.ConnectionTypeString, Label: "Kubernetes Version", Placeholder: "e.g. v1.30.1 (see Get Cluster Options for the supported list)", Required: true},
	{Name: "endpoint_subnet_ocid", Type: core.ConnectionTypeString, Label: "API Endpoint Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… the Kubernetes API endpoint lives in (recommended)"},
	{Name: "endpoint_public_ip", Type: core.ConnectionTypeBoolean, Label: "Public API Endpoint", Placeholder: "Give the Kubernetes API endpoint a public IP (only with an endpoint subnet)"},
	{Name: "cluster_type", Type: core.ConnectionTypeString, Label: "Cluster Type", Placeholder: "ENHANCED_CLUSTER (default) or BASIC_CLUSTER", Options: []core.ConnectionOption{
		{Name: "Enhanced", Value: "ENHANCED_CLUSTER"},
		{Name: "Basic", Value: "BASIC_CLUSTER"},
	}},
	{Name: "service_lb_subnet_ocids", Type: core.ConnectionTypeString, Label: "Service LB Subnet OCIDs", Placeholder: "Comma-separated subnet OCIDs for load-balancer services (optional)"},
	{Name: "kms_key_ocid", Type: core.ConnectionTypeString, Label: "KMS Key OCID", Placeholder: "ocid1.key.oc1..aaaa… to encrypt etcd/secrets (optional)"},
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
	name, err := oke.RequiredString("name", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	vcnID, err := oke.RequiredString("vcn_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	k8sVersion, err := oke.RequiredString("kubernetes_version", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	details := okesdk.CreateClusterDetails{
		Name:              &name,
		CompartmentId:     &compartment,
		VcnId:             &vcnID,
		KubernetesVersion: &k8sVersion,
		Type:              okesdk.ClusterTypeEnhancedCluster,
	}
	switch strings.ToUpper(strings.TrimSpace(oke.OptionalString("cluster_type", inputs))) {
	case "BASIC_CLUSTER", "BASIC":
		details.Type = okesdk.ClusterTypeBasicCluster
	case "", "ENHANCED_CLUSTER", "ENHANCED":
		details.Type = okesdk.ClusterTypeEnhancedCluster
	default:
		return oke.ErrorResult("cluster type must be ENHANCED_CLUSTER or BASIC_CLUSTER"), nil
	}
	if lbSubnets := oke.InputStrings("service_lb_subnet_ocids", inputs); len(lbSubnets) > 0 {
		details.Options = &okesdk.ClusterCreateOptions{ServiceLbSubnetIds: lbSubnets}
	}
	// A public API endpoint is only meaningful with an endpoint subnet — fail loudly rather
	// than silently dropping the flag when the subnet is blank.
	if oke.OptionalBool("endpoint_public_ip", inputs, false) && oke.OptionalString("endpoint_subnet_ocid", inputs) == "" {
		return oke.ErrorResult("a public API endpoint requires an endpoint subnet — set the API Endpoint Subnet OCID"), nil
	}
	if endpointSubnet := oke.OptionalString("endpoint_subnet_ocid", inputs); endpointSubnet != "" {
		ep := okesdk.CreateClusterEndpointConfigDetails{SubnetId: &endpointSubnet}
		if oke.BoolWasSet("endpoint_public_ip", inputs) {
			pub := oke.OptionalBool("endpoint_public_ip", inputs, false)
			ep.IsPublicIpEnabled = &pub
		}
		details.EndpointConfig = &ep
	}
	if kms := oke.OptionalString("kms_key_ocid", inputs); kms != "" {
		details.KmsKeyId = &kms
	}
	if tags, err := oke.FreeformTags("tags", inputs); err != nil {
		return oke.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	resp, err := client.CreateCluster(oke.Context(), okesdk.CreateClusterRequest{CreateClusterDetails: details})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	return oke.AsyncResult("Creating cluster "+name+" — poll Get Work Request until it completes", oke.Str(resp.OpcWorkRequestId)), nil
}
