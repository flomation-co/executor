package aws_ec2_describe_instances

import (
	"testing"
	"time"

	core "flomation.app/automate/executor"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/gomega"
)

// The AWS action template contract: it's an Action node, tool_result is the
// first output, and the standard credential block is present.
func TestMetadataAndContract(t *testing.T) {
	RegisterTestingT(t)

	Expect(Type).To(Equal(core.ActionTypeAction))
	Expect(Outputs[0].Name).To(Equal("tool_result"))

	names := map[string]bool{}
	for _, i := range Inputs {
		names[i.Name] = true
	}
	for _, want := range []string{"auth_method", "aws_access_key", "aws_secret_key", "aws_region", "aws_session_token", "assume_role_arn", "external_id", "instance_ids"} {
		Expect(names).To(HaveKey(want))
	}
}

func TestSummariseInstance(t *testing.T) {
	RegisterTestingT(t)

	lt := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	inst := types.Instance{
		InstanceId:       aws.String("i-0abc"),
		InstanceType:     types.InstanceTypeT3Micro,
		PrivateIpAddress: aws.String("10.0.0.5"),
		PublicIpAddress:  aws.String("52.1.2.3"),
		SubnetId:         aws.String("subnet-1"),
		VpcId:            aws.String("vpc-1"),
		ImageId:          aws.String("ami-1"),
		LaunchTime:       aws.Time(lt),
		State:            &types.InstanceState{Name: types.InstanceStateNameRunning},
		Placement:        &types.Placement{AvailabilityZone: aws.String("eu-west-2a")},
		Tags:             []types.Tag{{Key: aws.String("Name"), Value: aws.String("web-1")}},
	}

	m := summariseInstance(inst)
	Expect(m["instance_id"]).To(Equal("i-0abc"))
	Expect(m["instance_type"]).To(Equal("t3.micro"))
	Expect(m["state"]).To(Equal("running"))
	Expect(m["availability_zone"]).To(Equal("eu-west-2a"))
	Expect(m["name"]).To(Equal("web-1"))
	Expect(m["launch_time"]).To(Equal("2026-07-17T09:00:00Z"))
	Expect(m["tags"]).To(HaveKeyWithValue("Name", "web-1"))
}
