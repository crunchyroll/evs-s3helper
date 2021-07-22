package s3client

import (
	"io"
	"time"
)

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
