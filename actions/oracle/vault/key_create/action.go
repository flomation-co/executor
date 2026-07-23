// Package oracle_vault_key_create creates a master encryption key in a vault. Keys live
// at the vault's MANAGEMENT endpoint, so this resolves the vault first (via its OCID) and
// then talks to that endpoint. Poll Get Key until ENABLED.
package oracle_vault_key_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Create Key"
	Description  = "Create a master encryption key in an Oracle Cloud vault — used to encrypt/decrypt data and protect secrets. Pick the algorithm (AES/RSA/ECDSA); for ECDSA choose the curve. Defaults to a SOFTWARE-protected key; poll Get Key until ENABLED."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
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
	{Name: "vault_ocid", Type: core.ConnectionTypeString, Label: "Vault OCID", Placeholder: "ocid1.vault.oc1..aaaa… the key lives in", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A friendly name for the key", Required: true},
	{Name: "algorithm", Type: core.ConnectionTypeString, Label: "Algorithm", Placeholder: "AES (default), RSA or ECDSA", Options: []core.ConnectionOption{
		{Name: "AES (symmetric)", Value: "AES"},
		{Name: "RSA", Value: "RSA"},
		{Name: "ECDSA", Value: "ECDSA"},
	}},
	{Name: "curve_id", Type: core.ConnectionTypeString, Label: "Curve (ECDSA only)", Placeholder: "The elliptic curve for ECDSA keys — default NIST_P256", Options: []core.ConnectionOption{
		{Name: "NIST P-256", Value: "NIST_P256"},
		{Name: "NIST P-384", Value: "NIST_P384"},
		{Name: "NIST P-521", Value: "NIST_P521"},
	}},
	{Name: "length", Type: core.ConnectionTypeString, Label: "Length (bytes)", Placeholder: "Optional — AES 16/24/32 (default 32); RSA 256/384/512 (default 256); ECDSA is set by the curve"},
	{Name: "protection_mode", Type: core.ConnectionTypeString, Label: "Protection Mode", Placeholder: "SOFTWARE (default) or HSM", Options: []core.ConnectionOption{
		{Name: "Software", Value: "SOFTWARE"},
		{Name: "HSM", Value: "HSM"},
	}},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "key", Type: core.ConnectionTypeObject, Label: "Key"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Key OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, _, errResult := kms.ManagementForVault(inputs, "vault_ocid")
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	displayName, err := kms.RequiredString("display_name", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	algorithm := keymanagement.KeyShapeAlgorithmAes
	switch strings.ToUpper(strings.TrimSpace(kms.OptionalString("algorithm", inputs))) {
	case "RSA":
		algorithm = keymanagement.KeyShapeAlgorithmRsa
	case "ECDSA":
		algorithm = keymanagement.KeyShapeAlgorithmEcdsa
	case "", "AES":
		algorithm = keymanagement.KeyShapeAlgorithmAes
	default:
		return kms.ErrorResult("algorithm must be AES, RSA or ECDSA"), nil
	}
	// An explicit length wins; otherwise default per algorithm — a flat default of 32 is only
	// valid for AES, and RSA/ECDSA have their own length rules (ECDSA's is fixed by the curve).
	explicitLen, hasLen, err := kms.OptionalInt("length", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	shape := keymanagement.KeyShape{Algorithm: algorithm}
	switch algorithm {
	case keymanagement.KeyShapeAlgorithmRsa:
		length := 256
		if hasLen {
			length = explicitLen
		}
		shape.Length = &length
	case keymanagement.KeyShapeAlgorithmEcdsa:
		// ECDSA needs a curve, and its length is fixed by that curve (P-256→32, P-384→48, P-521→66).
		curve := keymanagement.KeyShapeCurveIdP256
		length := 32
		switch strings.ToUpper(strings.TrimSpace(kms.OptionalString("curve_id", inputs))) {
		case "NIST_P384", "P384":
			curve, length = keymanagement.KeyShapeCurveIdP384, 48
		case "NIST_P521", "P521":
			curve, length = keymanagement.KeyShapeCurveIdP521, 66
		case "", "NIST_P256", "P256":
			curve, length = keymanagement.KeyShapeCurveIdP256, 32
		default:
			return kms.ErrorResult("curve must be NIST_P256, NIST_P384 or NIST_P521"), nil
		}
		shape.CurveId = curve
		// A curve fixes the key length; catch a mismatched explicit length locally rather than
		// letting it surface as an opaque OCI 400.
		if hasLen && explicitLen != length {
			return kms.ErrorResult(fmt.Sprintf("ECDSA length must be %d bytes for %s (or leave Length blank to use the curve default)", length, curve)), nil
		}
		shape.Length = &length
	default: // AES
		length := 32
		if hasLen {
			length = explicitLen
		}
		shape.Length = &length
	}
	details := keymanagement.CreateKeyDetails{CompartmentId: &compartment, DisplayName: &displayName, KeyShape: &shape}
	// Default protection mode to SOFTWARE. OCI's own default for an omitted field is HSM, which
	// would silently consume a scarce HSM key slot and contradict this action's stated default.
	details.ProtectionMode = keymanagement.CreateKeyDetailsProtectionModeSoftware
	if strings.ToUpper(strings.TrimSpace(kms.OptionalString("protection_mode", inputs))) == "HSM" {
		details.ProtectionMode = keymanagement.CreateKeyDetailsProtectionModeHsm
	}
	if tags, err := kms.FreeformTags("tags", inputs); err != nil {
		return kms.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	resp, err := client.CreateKey(kms.Context(), keymanagement.CreateKeyRequest{CreateKeyDetails: details})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	key := kms.SummariseKey(&resp.Key)
	return kms.Result(fmt.Sprintf("Creating key %q (%s) — poll Get Key until ENABLED", displayName, key["lifecycle_state"]), map[string]interface{}{
		"key": key, "id": key["id"], "lifecycle_state": key["lifecycle_state"],
	}), nil
}
