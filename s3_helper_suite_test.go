package main_test

import (
        "testing"

        . "github.com/onsi/ginkgo/v2"
        . "github.com/onsi/gomega"
)

func TestS3Helper(t *testing.T) {
        RegisterFailHandler(Fail)
        RunSpecs(t, "S3-Helper Suite")
}
