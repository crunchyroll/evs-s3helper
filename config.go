package main

import "time"

// Config holds the global config
type Config struct {
	Listen string `yaml:"listen"`

	S3Bucket  string        `yaml:"s3_bucket"`
	S3Path    string        `yaml:"s3_prefix" optional:"true"`
	S3Region  string        `yaml:"s3_region"`
	S3Retries int           `yaml:"s3_retries"`
	S3Timeout time.Duration `yaml:"s3_timeout"`

	LogLevel    string `optional:"true"`
	Concurrency int    `optional:"true"`
}

const defaultConfValues = `
    listen: "127.0.0.1:8080"
    loglevel: "error"
    s3_timeout:  5s
    s3_retries:  5
    concurrency:   0
`
