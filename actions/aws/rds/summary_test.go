package rds

import (
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	. "github.com/onsi/gomega"
)

func TestSummariseInstance(t *testing.T) {
	RegisterTestingT(t)

	inst := &rdstypes.DBInstance{
		DBInstanceIdentifier: awssdk.String("my-db"),
		DBInstanceClass:      awssdk.String("db.t3.micro"),
		Engine:               awssdk.String("postgres"),
		DBInstanceStatus:     awssdk.String("available"),
		AllocatedStorage:     awssdk.Int32(20),
		PubliclyAccessible:   awssdk.Bool(false),
		Endpoint: &rdstypes.Endpoint{
			Address: awssdk.String("my-db.abc.eu-west-2.rds.amazonaws.com"),
			Port:    awssdk.Int32(5432),
		},
	}

	m := SummariseInstance(inst)
	Expect(m["db_instance_identifier"]).To(Equal("my-db"))
	Expect(m["engine"]).To(Equal("postgres"))
	Expect(m["status"]).To(Equal("available"))
	Expect(m["allocated_storage"]).To(Equal(int32(20)))
	Expect(m["endpoint_address"]).To(Equal("my-db.abc.eu-west-2.rds.amazonaws.com"))
	Expect(m["endpoint_port"]).To(Equal(int32(5432)))

	// Nil-safe.
	Expect(SummariseInstance(nil)).To(BeNil())
}

func TestSummariseInstanceWithoutEndpoint(t *testing.T) {
	RegisterTestingT(t)

	// A creating instance has no endpoint yet — must not panic or invent keys.
	m := SummariseInstance(&rdstypes.DBInstance{
		DBInstanceIdentifier: awssdk.String("pending-db"),
		DBInstanceStatus:     awssdk.String("creating"),
	})
	Expect(m["db_instance_identifier"]).To(Equal("pending-db"))
	_, hasAddr := m["endpoint_address"]
	Expect(hasAddr).To(BeFalse())
}

func TestSummariseSnapshot(t *testing.T) {
	RegisterTestingT(t)

	created := time.Date(2026, 7, 20, 9, 30, 0, 0, time.UTC)
	m := SummariseSnapshot(&rdstypes.DBSnapshot{
		DBSnapshotIdentifier: awssdk.String("my-db-snap"),
		DBInstanceIdentifier: awssdk.String("my-db"),
		Status:               awssdk.String("available"),
		SnapshotType:         awssdk.String("manual"),
		Engine:               awssdk.String("postgres"),
		AllocatedStorage:     awssdk.Int32(20),
		SnapshotCreateTime:   &created,
	})
	Expect(m["db_snapshot_identifier"]).To(Equal("my-db-snap"))
	Expect(m["snapshot_type"]).To(Equal("manual"))
	Expect(m["created_at"]).To(Equal("2026-07-20T09:30:00Z"))

	Expect(SummariseSnapshot(nil)).To(BeNil())
}

func TestSummariseCluster(t *testing.T) {
	RegisterTestingT(t)

	writer := true
	reader := false
	c := &rdstypes.DBCluster{
		DBClusterIdentifier: awssdk.String("my-aurora"),
		Status:              awssdk.String("available"),
		Engine:              awssdk.String("aurora-postgresql"),
		MultiAZ:             awssdk.Bool(true),
		Endpoint:            awssdk.String("my-aurora.cluster-abc.eu-west-2.rds.amazonaws.com"),
		ReaderEndpoint:      awssdk.String("my-aurora.cluster-ro-abc.eu-west-2.rds.amazonaws.com"),
		DBClusterMembers: []rdstypes.DBClusterMember{
			{DBInstanceIdentifier: awssdk.String("my-aurora-1"), IsClusterWriter: &writer},
			{DBInstanceIdentifier: awssdk.String("my-aurora-2"), IsClusterWriter: &reader},
		},
	}

	m := SummariseCluster(c)
	Expect(m["db_cluster_identifier"]).To(Equal("my-aurora"))
	Expect(m["multi_az"]).To(Equal(true))
	Expect(m["writer_endpoint"]).To(ContainSubstring("cluster-abc"))
	Expect(m["reader_endpoint"]).To(ContainSubstring("cluster-ro-abc"))

	members := m["members"].([]map[string]interface{})
	Expect(members).To(HaveLen(2))
	Expect(members[0]["db_instance_identifier"]).To(Equal("my-aurora-1"))
	Expect(members[0]["is_writer"]).To(Equal(true))
	Expect(members[1]["is_writer"]).To(Equal(false))

	Expect(SummariseCluster(nil)).To(BeNil())
}

func TestSummariseClusterSnapshot(t *testing.T) {
	RegisterTestingT(t)

	m := SummariseClusterSnapshot(&rdstypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: awssdk.String("my-aurora-snap"),
		DBClusterIdentifier:         awssdk.String("my-aurora"),
		Status:                      awssdk.String("available"),
		SnapshotType:                awssdk.String("manual"),
	})
	Expect(m["db_cluster_snapshot_identifier"]).To(Equal("my-aurora-snap"))
	Expect(m["db_cluster_identifier"]).To(Equal("my-aurora"))
	Expect(m["snapshot_type"]).To(Equal("manual"))

	Expect(SummariseClusterSnapshot(nil)).To(BeNil())
}
