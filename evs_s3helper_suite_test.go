package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestEvsS3helper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "main Suite")
}
