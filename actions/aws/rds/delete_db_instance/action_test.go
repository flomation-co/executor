package aws_rds_delete_db_instance

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func TestRequiresIdentifier(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("identifier"))
}

// With SkipFinalSnapshot off, AWS requires a final snapshot name. We validate
// that up front so the user gets a clear message instead of an opaque AWS error.
func TestRequiresFinalSnapshotWhenNotSkipping(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		str("db_instance_identifier", "my-db"),
		{Name: "skip_final_snapshot", Type: core.ConnectionTypeBoolean, Value: false},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("final snapshot"))
}
