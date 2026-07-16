// Shared plumbing for the azure/cosmosdb action tests (the per-resource-group
// files next door). Everything drives the real Execute functions against
// httptest servers — the endpoint credential input IS the test seam, so no
// package state needs swapping.
package cosmosdb_test

import (
	core "flomation.app/automate/executor"
)

// testMasterKey is the Cosmos DB emulator's well-known, public master key.
const testMasterKey = "C2y6yDjf5/R+ob0N8A7Cgv30VRDJIWEHLM+4QDU5DE2nQ9nDuVTqobD4b8mGGyPMbIZnqyMsEcaGQy67XIw/Jw=="

func str(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: v}
}

func text(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: v}
}

func secret(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeSecret, Value: v}
}

func boolean(name string, v bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: v}
}

func integer(name string, v int) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: v}
}

func object(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: v}
}

// authFor is the master-key credential block pointed at an httptest server.
func authFor(endpoint string, more ...*core.Connection) []*core.Connection {
	inputs := []*core.Connection{
		str("account_name", "testaccount"),
		secret("master_key", testMasterKey),
		str("endpoint", endpoint),
	}
	return append(inputs, more...)
}
