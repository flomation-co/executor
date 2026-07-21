package rds

import (
	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// SummariseInstance flattens the DBInstance fields most useful downstream into a
// plain map (so it serialises cleanly through node results). Shared by the
// describe and lifecycle actions so every RDS action reports an instance the
// same way.
func SummariseInstance(i *rdstypes.DBInstance) map[string]interface{} {
	if i == nil {
		return nil
	}
	m := map[string]interface{}{
		"db_instance_identifier": awssdk.ToString(i.DBInstanceIdentifier),
		"db_instance_class":      awssdk.ToString(i.DBInstanceClass),
		"engine":                 awssdk.ToString(i.Engine),
		"engine_version":         awssdk.ToString(i.EngineVersion),
		"status":                 awssdk.ToString(i.DBInstanceStatus),
		"master_username":        awssdk.ToString(i.MasterUsername),
		"availability_zone":      awssdk.ToString(i.AvailabilityZone),
		"multi_az":               awssdk.ToBool(i.MultiAZ),
		"publicly_accessible":    awssdk.ToBool(i.PubliclyAccessible),
		"allocated_storage":      awssdk.ToInt32(i.AllocatedStorage),
		"arn":                    awssdk.ToString(i.DBInstanceArn),
	}
	if i.Endpoint != nil {
		m["endpoint_address"] = awssdk.ToString(i.Endpoint.Address)
		m["endpoint_port"] = awssdk.ToInt32(i.Endpoint.Port)
	}
	return m
}

// SummariseSnapshot flattens the DBSnapshot fields most useful downstream.
func SummariseSnapshot(s *rdstypes.DBSnapshot) map[string]interface{} {
	if s == nil {
		return nil
	}
	m := map[string]interface{}{
		"db_snapshot_identifier": awssdk.ToString(s.DBSnapshotIdentifier),
		"db_instance_identifier": awssdk.ToString(s.DBInstanceIdentifier),
		"status":                 awssdk.ToString(s.Status),
		"snapshot_type":          awssdk.ToString(s.SnapshotType),
		"engine":                 awssdk.ToString(s.Engine),
		"engine_version":         awssdk.ToString(s.EngineVersion),
		"allocated_storage":      awssdk.ToInt32(s.AllocatedStorage),
		"arn":                    awssdk.ToString(s.DBSnapshotArn),
	}
	if s.SnapshotCreateTime != nil {
		m["created_at"] = s.SnapshotCreateTime.UTC().Format("2006-01-02T15:04:05Z")
	}
	return m
}

// SummariseCluster flattens an Aurora/RDS DB cluster, exposing both the writer
// endpoint and the reader endpoint and each member's writer/reader role — the
// read/write split that the instance-level actions can't express.
func SummariseCluster(c *rdstypes.DBCluster) map[string]interface{} {
	if c == nil {
		return nil
	}
	var members []map[string]interface{}
	for _, mem := range c.DBClusterMembers {
		members = append(members, map[string]interface{}{
			"db_instance_identifier": awssdk.ToString(mem.DBInstanceIdentifier),
			"is_writer":              awssdk.ToBool(mem.IsClusterWriter),
		})
	}
	return map[string]interface{}{
		"db_cluster_identifier": awssdk.ToString(c.DBClusterIdentifier),
		"status":                awssdk.ToString(c.Status),
		"engine":                awssdk.ToString(c.Engine),
		"engine_version":        awssdk.ToString(c.EngineVersion),
		"database_name":         awssdk.ToString(c.DatabaseName),
		"master_username":       awssdk.ToString(c.MasterUsername),
		"multi_az":              awssdk.ToBool(c.MultiAZ),
		"writer_endpoint":       awssdk.ToString(c.Endpoint),
		"reader_endpoint":       awssdk.ToString(c.ReaderEndpoint),
		"allocated_storage":     awssdk.ToInt32(c.AllocatedStorage),
		"arn":                   awssdk.ToString(c.DBClusterArn),
		"members":               members,
	}
}

// SummariseClusterSnapshot flattens a DB cluster snapshot.
func SummariseClusterSnapshot(s *rdstypes.DBClusterSnapshot) map[string]interface{} {
	if s == nil {
		return nil
	}
	m := map[string]interface{}{
		"db_cluster_snapshot_identifier": awssdk.ToString(s.DBClusterSnapshotIdentifier),
		"db_cluster_identifier":          awssdk.ToString(s.DBClusterIdentifier),
		"status":                         awssdk.ToString(s.Status),
		"snapshot_type":                  awssdk.ToString(s.SnapshotType),
		"engine":                         awssdk.ToString(s.Engine),
		"engine_version":                 awssdk.ToString(s.EngineVersion),
		"allocated_storage":              awssdk.ToInt32(s.AllocatedStorage),
		"arn":                            awssdk.ToString(s.DBClusterSnapshotArn),
	}
	if s.SnapshotCreateTime != nil {
		m["created_at"] = s.SnapshotCreateTime.UTC().Format("2006-01-02T15:04:05Z")
	}
	return m
}
