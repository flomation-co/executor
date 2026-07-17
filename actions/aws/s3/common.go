package s3

import (
	"context"

	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Service struct {
	Client *s3.Client
}

// GetService builds an S3 client via the shared AWS config helper, so S3 gets
// the same credential handling (and optional assume-role) as every other AWS
// action. Region is required — callers default a blank region to eu-west-2 for
// backwards compatibility with flows saved before the region input existed.
func GetService(accessKey string, secretKey string, region string) (*Service, error) {
	cfg, err := awscommon.Config(context.Background(), awscommon.Credentials{
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    region,
	})
	if err != nil {
		return nil, err
	}

	return &Service{Client: s3.NewFromConfig(cfg)}, nil
}
