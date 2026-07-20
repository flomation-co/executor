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
