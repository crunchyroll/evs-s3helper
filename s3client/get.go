package s3client

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

// GetObject - talks to S3 to get content/byte-range from the bucket
func (client *S3Client) GetObject(bucket, s3Path, byterange string) (*GetObjectOutput, error) {
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(s3Path),
		Range:  aws.String(byterange),
	}

	result, err := client.s3Manager.GetObject(getInput)
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
