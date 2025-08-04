// Package cloudflare provides a client for interacting with the Cloudflare API.
package cloudflare

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

type R2Client struct {
	C      *s3.Client
	Bucket *string
}

func NewR2() (*R2Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			viper.GetString("cloudflare.access_key_id"),
			viper.GetString("cloudflare.secret_access_key"),
			"",
		)),
	)
	if err != nil {
		return nil, err
	}

	bucket := aws.String(viper.GetString("cloudflare.bucket"))

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", viper.GetString("cloudflare.account_id")))
		o.Region = "auto"
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

	return &R2Client{
		C:      client,
		Bucket: bucket,
	}, nil
}
