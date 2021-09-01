package awsclient

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

type media struct {
	cmsId     string
	pHash     string
	createdAt string
}

func TestS3Client_Success(t *testing.T) {
	dummyFiles := map[string]interface{}{
		"avod/mediaId-0": struct{ cmsId string }{"A"},
		"avod/mediaId-1": media{cmsId: "A", pHash: "abcd-12345", createdAt: "01/01/2021"},
		"avod/mediaId-2": media{cmsId: "B", pHash: "abcd-1234", createdAt: "02/02/2021"},
		"svod/mediaId-0": struct{ cmsId string }{"A"},
		"svod/mediaId-1": media{cmsId: "A", pHash: "abcd-12345", createdAt: "01/01/2021"},
		"svod/mediaId-2": media{cmsId: "B", pHash: "abcd-1234", createdAt: "02/02/2021"},
	}

	mS3 := mockS3Client{files: dummyFiles}
	var getInput *s3.GetObjectInput
	var getOutput *s3.GetObjectOutput
	var err error

	getInput = &s3.GetObjectInput{Bucket: aws.String("avod"), Key: aws.String("mediaId-1")}
	getOutput, err = mS3.GetObject(getInput)
	if err != nil {
		t.Fatalf("S3:GetObject response has error: %v", err)
	}

	if len(*getOutput.ETag) == 0 {
		t.Fatalf("S3:GetObject response has empty etag: %v", *getOutput)
	}

	if *getOutput.ContentLength == 0 {
		t.FailNow()
	}

	getInput = &s3.GetObjectInput{Bucket: aws.String("avod"), Key: aws.String("mediaId-0")}
	getOutput, err = mS3.GetObject(getInput)
	if err != nil {
		t.Fatalf("S3:GetObject response has error: %v", err)
	}

	if len(*getOutput.ETag) == 0 {
		t.Fatalf("S3:GetObject response has empty etag: %v", *getOutput)
	}

	if *getOutput.ContentLength == 0 {
		t.FailNow()
	}

	getInput = &s3.GetObjectInput{Bucket: aws.String("svod"), Key: aws.String("mediaId-1")}
	getOutput, err = mS3.GetObject(getInput)
	if err != nil {
		t.Fatalf("S3:GetObject response has error: %v", err)
	}

	if len(*getOutput.ETag) == 0 {
		t.Fatalf("S3:GetObject response has empty etag: %v", *getOutput)
	}

	if *getOutput.ContentLength == 0 {
		t.FailNow()
	}

	getInput = &s3.GetObjectInput{Bucket: aws.String("svod"), Key: aws.String("mediaId-0")}
	getOutput, err = mS3.GetObject(getInput)
	if err != nil {
		t.Fatalf("S3:GetObject response has error: %v", err)
	}

	if len(*getOutput.ETag) == 0 {
		t.Fatalf("S3:GetObject response has empty etag: %v", *getOutput)
	}

	if *getOutput.ContentLength == 0 {
		t.FailNow()
	}
}

func TestS3Client_Failure(t *testing.T) {
	dummyFiles := map[string]interface{}{
		"avod/mediaId-0": struct{ cmsId string }{"A"},
		"avod/mediaId-1": media{cmsId: "A", pHash: "abcd-12345", createdAt: "01/01/2021"},
		"avod/mediaId-2": media{cmsId: "B", pHash: "abcd-1234", createdAt: "02/02/2021"},
		"svod/mediaId-0": struct{ cmsId string }{"A"},
		"svod/mediaId-1": media{cmsId: "A", pHash: "abcd-12345", createdAt: "01/01/2021"},
		"svod/mediaId-2": media{cmsId: "B", pHash: "abcd-1234", createdAt: "02/02/2021"},
	}

	mS3 := mockS3Client{files: dummyFiles}
	var getInput *s3.GetObjectInput
	// var getOutput *s3.GetObjectOutput
	var err error

	getInput = &s3.GetObjectInput{Bucket: aws.String("avod"), Key: aws.String("mediaId-3")}
	_, err = mS3.GetObject(getInput)

	// this should fail, as the key is not present
	if err == nil {
		t.FailNow()
	}

	/*
		fmt.Printf("getOutput: %+v", getOutput)
		if len(*getOutput.ETag) == 0 {
			t.FailNow()
		}

		if *getOutput.ContentLength != 0 {
			t.FailNow()
		}
	*/
	getInput = &s3.GetObjectInput{Bucket: aws.String("avod"), Key: aws.String("mediaId-3"), Range: aws.String("0-10")}
	_, err = mS3.GetObject(getInput)

	// this should fail, as the key is not present
	if err == nil {
		t.FailNow()
	}
}
