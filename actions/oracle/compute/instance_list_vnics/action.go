// Package oracle_compute_instance_list_vnics resolves an OCI Compute instance's
// attached VNICs to their private/public IPs — the "how do I reach this box"
// lookup. It joins two calls: ListVnicAttachments (Compute) gives the VNIC OCIDs,
// then GetVnic (Network) resolves each to its addresses.
package oracle_compute_instance_list_vnics

import (
	"fmt"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/oracle/compute"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Compute: List Instance VNICs"
	Description  = "List an instance's attached VNICs with their private and public IP addresses — how to reach the instance. Resolves each VNIC attachment to its IPs."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+ethernet"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "The instance's compartment OCID", Required: true},
	{Name: "instance_ocid", Type: core.ConnectionTypeString, Label: "Instance OCID", Placeholder: "ocid1.instance.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vnics", Type: core.ConnectionTypeObject, Label: "VNICs"},
	{Name: "primary_public_ip", Type: core.ConnectionTypeString, Label: "Primary Public IP"},
	{Name: "primary_private_ip", Type: core.ConnectionTypeString, Label: "Primary Private IP"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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
	instanceID, err := compute.RequiredString("instance_ocid", inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	computeClient, err := auth.ComputeClient()
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	netClient, err := auth.NetworkClient()
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := compute.Context()

	req := ocicore.ListVnicAttachmentsRequest{
		CompartmentId: compute.StringPtr(compartment),
		InstanceId:    &instanceID,
	}
	var vnics []map[string]interface{}
	var primaryPublic, primaryPrivate string
	var total, resolved int
	var lastResolveErr string
	for {
		resp, err := computeClient.ListVnicAttachments(ctx, req)
		if err != nil {
			return compute.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			att := &resp.Items[i]
			if att.VnicId == nil {
				continue // attachment still provisioning — no VNIC yet
			}
			total++
			// Uniform item shape: every VNIC carries the same keys, defaulted, so a
			// resolution failure yields blanks rather than a missing key downstream.
			v := map[string]interface{}{
				"vnic_id":        compute.Str(att.VnicId),
				"subnet_id":      compute.Str(att.SubnetId),
				"private_ip":     "",
				"public_ip":      "",
				"hostname_label": "",
				"is_primary":     false,
			}
			if vn, err := netClient.GetVnic(ctx, ocicore.GetVnicRequest{VnicId: att.VnicId}); err == nil {
				resolved++
				v["private_ip"] = compute.Str(vn.PrivateIp)
				v["public_ip"] = compute.Str(vn.PublicIp)
				v["hostname_label"] = compute.Str(vn.HostnameLabel)
				isPrimary := vn.IsPrimary != nil && *vn.IsPrimary
				v["is_primary"] = isPrimary
				if isPrimary {
					primaryPublic = compute.Str(vn.PublicIp)
					primaryPrivate = compute.Str(vn.PrivateIp)
				}
			} else {
				lastResolveErr = auth.OCIError(err)
			}
			vnics = append(vnics, v)
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	// If the instance has VNIC attachments but NONE resolved to their addresses,
	// that's a systematic failure (commonly the API key's user missing the
	// "use virtual-network-family" policy) — surface it rather than returning a
	// green node with blank IPs, which reads as "the instance has no public IP".
	if total > 0 && resolved == 0 {
		return compute.ErrorResult(fmt.Sprintf("found %d VNIC attachment(s) but could not read any of their addresses — the API key's user likely lacks virtual-network read access (Last error: %s)", total, lastResolveErr)), nil
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Instance has %d VNIC(s); primary public IP %q", len(vnics), primaryPublic),
		"vnics":              vnics,
		"primary_public_ip":  primaryPublic,
		"primary_private_ip": primaryPrivate,
		"count":              len(vnics),
		"success":            true,
	}, nil
}
