// Package oracle_compute_instance_launch creates (launches) a new OCI Compute
// instance from an image into a subnet. It is the create action: it needs the
// availability domain, shape, image and subnet an operator gets from the
// List Availability Domains / Shapes / Images / Subnets actions.
package oracle_compute_instance_launch

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/oracle/compute"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Compute: Launch Instance"
	Description  = "Create (launch) a new Oracle Cloud Compute instance from an image into a subnet. Supply the availability domain, shape, image and subnet; for flexible shapes set the OCPUs and memory."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+plus"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (from List Availability Domains)", Required: true},
	{Name: "shape", Type: core.ConnectionTypeString, Label: "Shape", Placeholder: "e.g. VM.Standard.E5.Flex (from List Shapes)", Required: true},
	{Name: "image_ocid", Type: core.ConnectionTypeString, Label: "Image OCID", Placeholder: "ocid1.image.oc1..aaaa… (from List Images)", Required: true},
	{Name: "subnet_ocid", Type: core.ConnectionTypeString, Label: "Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… (from List Subnets)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A friendly name for the instance (optional)"},
	{Name: "ocpus", Type: core.ConnectionTypeString, Label: "OCPUs (flex shapes)", Placeholder: "e.g. 1 — required for .Flex shapes"},
	{Name: "memory_in_gbs", Type: core.ConnectionTypeString, Label: "Memory GB (flex shapes)", Placeholder: "e.g. 16 — required for .Flex shapes"},
	{Name: "boot_volume_size_in_gbs", Type: core.ConnectionTypeString, Label: "Boot Volume Size (GB)", Placeholder: "Override the image default (optional, min 50)"},
	{Name: "ssh_authorized_key", Type: core.ConnectionTypeText, Label: "SSH Public Key", Placeholder: "ssh-rsa AAAA… — public key for the default OS user (optional)"},
	{Name: "assign_public_ip", Type: core.ConnectionTypeBoolean, Label: "Assign Public IP", Placeholder: "Give the primary VNIC a public IP (subnet must be public)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance", Type: core.ConnectionTypeObject, Label: "Instance"},
	{Name: "instance_ocid", Type: core.ConnectionTypeString, Label: "Instance OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	ad, err := compute.RequiredString("availability_domain", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	shape, err := compute.RequiredString("shape", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	imageID, err := compute.RequiredString("image_ocid", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	subnetID, err := compute.RequiredString("subnet_ocid", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.ComputeClient()
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}

	source := ocicore.InstanceSourceViaImageDetails{ImageId: &imageID}
	if bv, ok, err := compute.OptionalFloat32("boot_volume_size_in_gbs", inputs); err != nil {
		return compute.ErrorResult(err.Error()), nil
	} else if ok {
		size := int64(bv)
		source.BootVolumeSizeInGBs = &size
	}

	vnic := &ocicore.CreateVnicDetails{SubnetId: &subnetID}
	assignPublic := compute.OptionalBool("assign_public_ip", inputs, false)
	vnic.AssignPublicIp = &assignPublic

	details := ocicore.LaunchInstanceDetails{
		CompartmentId:      &compartment,
		AvailabilityDomain: &ad,
		Shape:              &shape,
		SourceDetails:      source,
		CreateVnicDetails:  vnic,
	}
	if dn := compute.OptionalString("display_name", inputs); dn != "" {
		details.DisplayName = &dn
	}
	if key := compute.OptionalString("ssh_authorized_key", inputs); key != "" {
		details.Metadata = map[string]string{"ssh_authorized_keys": key}
	}
	tags, err := compute.FreeformTags("tags", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	if tags != nil {
		details.FreeformTags = tags
	}

	// Flexible shapes (.Flex) require an explicit OCPU/memory config; fixed shapes
	// reject one. Only attach ShapeConfig when the operator supplied a value.
	ocpus, hasOcpus, err := compute.OptionalFloat32("ocpus", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	mem, hasMem, err := compute.OptionalFloat32("memory_in_gbs", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	if hasOcpus || hasMem {
		sc := &ocicore.LaunchInstanceShapeConfigDetails{}
		if hasOcpus {
			sc.Ocpus = &ocpus
		}
		if hasMem {
			sc.MemoryInGBs = &mem
		}
		details.ShapeConfig = sc
	}

	resp, err := client.LaunchInstance(compute.Context(), ocicore.LaunchInstanceRequest{LaunchInstanceDetails: details})
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Launched instance %q (%s)", compute.Str(resp.DisplayName), compute.Str(resp.Id)),
		"instance":        compute.SummariseInstance(&resp.Instance),
		"instance_ocid":   compute.Str(resp.Id),
		"lifecycle_state": string(resp.LifecycleState),
		"success":         true,
	}, nil
}
