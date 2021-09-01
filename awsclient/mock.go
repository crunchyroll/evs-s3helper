package awsclient

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

type mockS3Client struct {
	s3iface.S3API
	files map[string]interface{}
	// files map[string][]byte this is for later
}

func (m *mockS3Client) GetObject(in *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	key := path.Join(*in.Bucket, *in.Key)
	if _, ok := m.files[key]; !ok {
		return &s3.GetObjectOutput{}, errors.New("Key does not exist")
	}
	cLength := int64(1)
	etag := "this-is-a-dummy-etag"
	return &s3.GetObjectOutput{
		Body:          ioutil.NopCloser(bytes.NewReader([]byte(fmt.Sprintf("%v", m.files[key])))),
		ContentLength: &cLength,
		ETag:          &etag,
	}, nil
}
