// Package oracle_bastion_bastion_create creates a bastion — the managed jump host that brokers
// restricted, time-limited SSH access from allow-listed client IPs to targets in a private subnet.
// Asynchronous: the bastion comes back CREATING with a work-request id; poll the Get Bastion action
// until it is ACTIVE before opening sessions.
package oracle_bastion_bastion_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	bas "flomation.app/automate/executor/actions/oracle/bastion"

	"github.com/oracle/oci-go-sdk/v65/bastion"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Bastion: Create Bastion"
	Description  = "Create a standard bastion in a target subnet. Optionally set a name, a client CIDR allow-list and a maximum session TTL. Returns a work-request id — poll Get Bastion until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+terminal"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "target_subnet_ocid", Type: core.ConnectionTypeString, Label: "Target Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… (private subnet the bastion connects to)", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "A name for the bastion, fixed after creation (optional)"},
	{Name: "max_session_ttl_seconds", Type: core.ConnectionTypeString, Label: "Max Session TTL (seconds)", Placeholder: "Longest a session may stay active, e.g. 10800 (optional)"},
	{Name: "client_cidr_allow_list", Type: core.ConnectionTypeString, Label: "Client CIDR Allow-list", Placeholder: "Comma-separated CIDRs allowed to connect, e.g. 10.0.0.0/24,203.0.113.5/32 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "bastion", Type: core.ConnectionTypeObject, Label: "Bastion"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Bastion OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := bas.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}
	subnet, err := bas.RequiredString("target_subnet_ocid", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}

	bastionType := "standard"
	details := bastion.CreateBastionDetails{
		BastionType:    &bastionType,
		CompartmentId:  &compartment,
		TargetSubnetId: &subnet,
	}
	if name := strings.TrimSpace(bas.OptionalString("name", inputs)); name != "" {
		details.Name = &name
	}
	ttl, err := bas.OptionalInt("max_session_ttl_seconds", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}
	if ttl != nil {
		details.MaxSessionTtlInSeconds = ttl
	}
	if raw := strings.TrimSpace(bas.OptionalString("client_cidr_allow_list", inputs)); raw != "" {
		var cidrs []string
		for _, part := range strings.Split(raw, ",") {
			if c := strings.TrimSpace(part); c != "" {
				cidrs = append(cidrs, c)
			}
		}
		details.ClientCidrBlockAllowList = cidrs
	}

	resp, err := client.CreateBastion(bas.Context(), bastion.CreateBastionRequest{CreateBastionDetails: details})
	if err != nil {
		return bas.ErrorResult(auth.OCIError(err)), nil
	}
	return bas.Result(fmt.Sprintf("Creating bastion in subnet %q — poll Get Bastion until ACTIVE", subnet), map[string]interface{}{
		"bastion":         bas.SummariseBastion(&resp.Bastion),
		"id":              bas.Str(resp.Bastion.Id),
		"lifecycle_state": string(resp.Bastion.LifecycleState),
		"work_request_id": bas.Str(resp.OpcWorkRequestId),
	}), nil
}
