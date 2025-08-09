// Package aws defines functions used to interact with the AWS API
package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/spf13/viper"
)

type S3Client struct {
	C      *s3.Client
	Bucket *string
}

func NewS3() (*S3Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			viper.GetString("aws.access_key"),
			viper.GetString("aws.secret_access_key"),
			"",
		)),
	)
	if err != nil {
		return nil, err
	}

	bucket := aws.String(viper.GetString("aws.bucket"))

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.Region = viper.GetString("aws.region")
	})

	_, err = client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: bucket,
	})
	if err != nil {
		var apiErr smithy.APIError

		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NotFound" {
				return nil, fmt.Errorf("bucket '%s' does not exist", viper.GetString("s3.bucket"))
			}
		}

		return nil, fmt.Errorf("failed to check if bucket exists, %w", err)
	}

	return &S3Client{
		C:      client,
		Bucket: bucket,
	}, nil
}
