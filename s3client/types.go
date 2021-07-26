package s3client

import (
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// S3Client - manages a persistent connection with downstream S3 bucket
type S3Client struct {
	s3Manager *s3.S3
}

// NewS3Client -  creates a new instance for S3Client with a aws session manager
// embedded inside.
// The objective of S3Client will allow callers to manager a persistent connection
// for a given bucket through it's life-time.
func NewS3Client(region string) (*S3Client, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config:            aws.Config{Region: aws.String(region)},
		SharedConfigState: session.SharedConfigEnable,
	})

	if err != nil {
		return &S3Client{}, fmt.Errorf("Failed to initiate an S3Client. Error: %+v", err)
	}

	return &S3Client{
		s3Manager: s3.New(sess),
	}, nil
}

// GetObjectOutput- constructs the output result from S3 GetObject call
type GetObjectOutput struct {
	Body            io.ReadCloser `type:"blob"`
	CacheControl    *string       `location:"header" locationName:"Cache-Control" type:"string"`
	ContentEncoding *string       `location:"header" locationName:"Content-Encoding" type:"string"`
	ContentLength   *int64        `location:"header" locationName:"Content-Length" type:"long"`
	ContentRange    *string       `location:"header" locationName:"Content-Range" type:"string"`
	ContentType     *string       `location:"header" locationName:"Content-Type" type:"string"`
	ETag            *string       `location:"header" locationName:"ETag" type:"string"`
	Expiration      *string       `location:"header" locationName:"x-amz-expiration" type:"string"`
	LastModified    *time.Time    `location:"header" locationName:"Last-Modified" type:"timestamp"`
	StorageClass    *string       `location:"header" locationName:"x-amz-storage-class" type:"string" enum:"StorageClass"`
	TagCount        *int64        `location:"header" locationName:"x-amz-tagging-count" type:"integer"`
	VersionId       *string       `location:"header" locationName:"x-amz-version-id" type:"string"`
}
