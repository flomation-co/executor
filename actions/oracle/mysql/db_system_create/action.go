// Package oracle_mysql_db_system_create provisions a new MySQL HeatWave DB system. Asynchronous:
// the create call returns a work-request id and the DB system comes up in a CREATING state — poll
// Get DB System until it is ACTIVE before connecting to it.
package oracle_mysql_db_system_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	my "flomation.app/automate/executor/actions/oracle/mysql"

	"github.com/oracle/oci-go-sdk/v65/mysql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI MySQL: Create DB System"
	Description  = "Provision a MySQL HeatWave DB system in a compartment — pick the shape, subnet and admin credentials. Returns a work-request id; the DB system starts in a CREATING state, so poll Get DB System until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+database"
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
	{Name: "shape_name", Type: core.ConnectionTypeString, Label: "Shape Name", Placeholder: "e.g. MySQL.2 — from List Shapes", Required: true},
	{Name: "subnet_ocid", Type: core.ConnectionTypeString, Label: "Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa…", Required: true},
	{Name: "admin_username", Type: core.ConnectionTypeString, Label: "Admin Username", Placeholder: "The administrative user to create", Required: true},
	{Name: "admin_password", Type: core.ConnectionTypeSecret, Label: "Admin Password", Placeholder: "8–32 chars, incl. upper, lower, number and special", Required: true},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. AaBb:UK-LONDON-1-AD-1 (optional)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the DB system (optional)"},
	{Name: "data_storage_gb", Type: core.ConnectionTypeString, Label: "Data Storage (GB)", Placeholder: "Initial data volume size in GBs (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := my.DbSystemClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}
	shape, err := my.RequiredString("shape_name", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}
	subnet, err := my.RequiredString("subnet_ocid", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}
	adminUser, err := my.RequiredString("admin_username", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}
	adminPass, err := my.RequiredString("admin_password", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}

	details := mysql.CreateDbSystemDetails{
		CompartmentId: &compartment,
		ShapeName:     &shape,
		SubnetId:      &subnet,
		AdminUsername: &adminUser,
		AdminPassword: &adminPass,
	}
	if ad := my.OptionalString("availability_domain", inputs); ad != "" {
		details.AvailabilityDomain = &ad
	}
	if name := my.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}
	if n, ok, err := my.OptionalInt("data_storage_gb", inputs); err != nil {
		return my.ErrorResult(err.Error()), nil
	} else if ok {
		details.DataStorageSizeInGBs = &n
	}

	resp, err := client.CreateDbSystem(my.Context(), mysql.CreateDbSystemRequest{CreateDbSystemDetails: details})
	if err != nil {
		return my.ErrorResult(auth.OCIError(err)), nil
	}
	return my.Result(fmt.Sprintf("Creating MySQL DB system %q — poll Get DB System until ACTIVE", my.Str(details.DisplayName)), map[string]interface{}{
		"work_request_id": my.Str(resp.OpcWorkRequestId),
	}), nil
}
