// Package oracle_bastion_session_create creates a managed SSH bastion session — a restricted,
// time-limited SSH connection from an allow-listed client to a target host that has no public
// endpoint. Asynchronous: the session comes back CREATING with a work-request id; poll Get Session
// until it is ACTIVE before connecting. The target resource must run an OpenSSH server and the
// Oracle Cloud Agent for a managed SSH session to succeed.
package oracle_bastion_session_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	bas "flomation.app/automate/executor/actions/oracle/bastion"

	"github.com/oracle/oci-go-sdk/v65/bastion"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Bastion: Create Session"
	Description  = "Open a managed SSH bastion session to a target host on a private subnet. Returns the session in a CREATING state plus a work-request id — poll Get Session until ACTIVE, then connect with the matching private key."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+terminal"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "bastion_ocid", Type: core.ConnectionTypeString, Label: "Bastion OCID", Placeholder: "ocid1.bastion.oc1..aaaa… the bastion to create the session on", Required: true},
	{Name: "target_resource_ocid", Type: core.ConnectionTypeString, Label: "Target Resource OCID", Placeholder: "ocid1.instance.oc1..aaaa… the host to connect to", Required: true},
	{Name: "target_os_username", Type: core.ConnectionTypeString, Label: "Target OS Username", Placeholder: "e.g. opc — the OS user the session logs in as", Required: true},
	{Name: "public_key", Type: core.ConnectionTypeText, Label: "SSH Public Key", Placeholder: "The OpenSSH public key (ssh-rsa AAAA…) whose private key you will connect with", Required: true},
	{Name: "target_resource_port", Type: core.ConnectionTypeString, Label: "Target Resource Port", Placeholder: "Port to connect to on the target (optional, default 22)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the session (optional)"},
	{Name: "session_ttl_seconds", Type: core.ConnectionTypeString, Label: "Session TTL (seconds)", Placeholder: "How long the session stays active (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "session", Type: core.ConnectionTypeObject, Label: "Session"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Session OCID"},
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

	bastionID, err := bas.RequiredString("bastion_ocid", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}
	targetID, err := bas.RequiredString("target_resource_ocid", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}
	osUser, err := bas.RequiredString("target_os_username", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}
	publicKey, err := bas.RequiredString("public_key", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}

	target := bastion.CreateManagedSshSessionTargetResourceDetails{
		TargetResourceId:                      &targetID,
		TargetResourceOperatingSystemUserName: &osUser,
	}
	if port, err := bas.OptionalInt("target_resource_port", inputs); err != nil {
		return bas.ErrorResult(err.Error()), nil
	} else if port != nil {
		target.TargetResourcePort = port
	}

	details := bastion.CreateSessionDetails{
		BastionId:             &bastionID,
		TargetResourceDetails: target,
		KeyDetails:            &bastion.PublicKeyDetails{PublicKeyContent: &publicKey},
	}
	if name := bas.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}
	if ttl, err := bas.OptionalInt("session_ttl_seconds", inputs); err != nil {
		return bas.ErrorResult(err.Error()), nil
	} else if ttl != nil {
		details.SessionTtlInSeconds = ttl
	}

	resp, err := client.CreateSession(bas.Context(), bastion.CreateSessionRequest{CreateSessionDetails: details})
	if err != nil {
		return bas.ErrorResult(auth.OCIError(err)), nil
	}

	summary := bas.SummariseSession(&resp.Session)
	return bas.Result(fmt.Sprintf("Creating managed SSH session on bastion %s — poll Get Session until ACTIVE", bastionID), map[string]interface{}{
		"session":         summary,
		"id":              summary["id"],
		"lifecycle_state": summary["lifecycle_state"],
		"work_request_id": bas.Str(resp.OpcWorkRequestId),
	}), nil
}
