package s3client_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
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
		return nil, errors.New("Key does not exist")
	}

	return &s3.GetObjectOutput{
		Body: ioutil.NopCloser(bytes.NewReader([]byte(fmt.Sprintf("%v", m.files[key])))),
	}, nil
}

func TestS3Client_Success(t *testing.T) {
	dummyFiles := map[string]interface{}{
		"avod/mediaId-1": struct{ cmsId string }{"A"},
		"avod/mediaId-2": struct{ cmsId string }{"B"},
	}

	mS3 := mockS3Client{files: dummyFiles}
	getInput := &s3.GetObjectInput{Bucket: aws.String("avod"), Key: aws.String("mediaId-1")}
	_, err := mS3.GetObject(getInput)
	if err != nil {
		t.FailNow()
	}
}

func TestS3Client_Failure(t *testing.T) {
	dummyFiles := map[string]interface{}{
		"avod/mediaId-1": struct{ cmsId string }{"A"},
		"avod/mediaId-2": struct{ cmsId string }{"B"},
	}

	mS3 := mockS3Client{files: dummyFiles}
	getInput := &s3.GetObjectInput{Bucket: aws.String("avod"), Key: aws.String("mediaId-3")}
	_, err := mS3.GetObject(getInput)

	// this should fail, as the key is not present
	if err == nil {
		t.FailNow()
	}
}
