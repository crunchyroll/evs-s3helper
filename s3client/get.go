package s3client

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// GetObject - talks to S3 to get content/byte-range from the bucket
func GetObject(bucket, s3Path, byterange string) (*GetObjectOutput, error) {
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(s3Path),
		Range:  aws.String(byterange),
	}

	sess, err := session.NewSessionWithOptions(session.Options{
		Config:            aws.Config{Region: aws.String("us-west-2")},
		SharedConfigState: session.SharedConfigEnable,
	})

	s3Manager := s3.New(sess)
	result, err := s3Manager.GetObject(getInput)
	if err != nil {
		return &GetObjectOutput{}, err
	}

	return &GetObjectOutput{
		Body:            result.Body,
		CacheControl:    result.CacheControl,
		ContentEncoding: result.ContentEncoding,
		ContentLength:   result.ContentLength,
		ContentRange:    result.ContentRange,
		ContentType:     result.ContentType,
		ETag:            result.ETag,
		Expiration:      result.Expiration,
		LastModified:    result.LastModified,
		StorageClass:    result.StorageClass,
		TagCount:        result.TagCount,
		VersionId:       result.VersionId}, nil
}
