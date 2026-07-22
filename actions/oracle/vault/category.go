package vault

// Sub-category metadata for the Vault / KMS provider under Oracle Cloud. The middle path
// segment "vault" nests every oracle/vault/<verb> action under this sub-group. The api
// recomputes display metadata from its own in-code maps at serve time
// (subCategoryMetadata), so these are for manifest completeness — the Description MUST
// stay byte-identical to the api's subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Vault"
	CategoryIcon        = "lock"
	CategoryDescription = "Oracle Cloud Vault & KMS — manage vaults, master encryption keys and key versions, run crypto operations (encrypt/decrypt/sign/verify), and store, rotate and retrieve secrets"
)
