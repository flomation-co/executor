package containerengine

// Sub-category metadata for the Container Engine (OKE) provider under Oracle Cloud. The
// middle path segment "containerengine" nests every oracle/containerengine/<verb> action
// under this sub-group. The api recomputes display metadata from its own in-code maps at
// serve time (subCategoryMetadata), so these are for manifest completeness — the
// Description MUST stay byte-identical to the api's subCategoryMetadata entry or the
// palette header drifts.
const (
	CategoryName        = "Container Engine"
	CategoryIcon        = "cubes"
	CategoryDescription = "Oracle Cloud Container Engine for Kubernetes (OKE) — provision and manage clusters, node pools and virtual node pools, install add-ons, generate kubeconfig, rotate credentials, and track asynchronous work requests"
)
