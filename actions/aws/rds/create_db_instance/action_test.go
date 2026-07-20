package aws_rds_create_db_instance

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// The required-field guard runs before any AWS call, so we can assert it without
// credentials or network. A partial input (no storage, no password) must fail.
func TestRequiresCoreFields(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		str("db_instance_identifier", "my-db"),
		str("db_instance_class", "db.t3.micro"),
		str("engine", "postgres"),
		str("master_username", "admin"),
		// no master_password, no allocated_storage
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("required"))
}

func TestBuildTagsSkipsBlankKeys(t *testing.T) {
	RegisterTestingT(t)

	// key_value_array values are stored as a JSON string at runtime.
	tags := buildTags([]*core.Connection{
		{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Value: `[{"key":"env","value":"prod"},{"key":"","value":"ignored"},{"key":"team","value":""}]`},
	})
	Expect(tags).To(HaveLen(2))
	Expect(*tags[0].Key).To(Equal("env"))
	Expect(*tags[0].Value).To(Equal("prod"))
	Expect(*tags[1].Key).To(Equal("team"))
}
