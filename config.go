package main

import "time"

type nrConfig struct {
	Name    string `yaml:"name"`
	License string `yaml:"license"`
}

// Config holds the global config
type Config struct {
	Listen string `yaml:"listen"`

	S3AdBucket string        `yaml:"s3_ad_bucket"`
	S3Bucket   string        `yaml:"s3_bucket"`
	S3Path     string        `yaml:"s3_prefix" optional:"true"`
	S3Region   string        `yaml:"s3_region"`
	S3Retries  int           `yaml:"s3_retries"`
	S3Timeout  time.Duration `yaml:"s3_timeout"`

	LogLevel    string `yaml:"loglevel"    optional:"true"`
	Concurrency int    `yaml:"concurrency" optional:"true"`

	NewRelic nrConfig `yaml:"newrelic"`
}

const defaultConfValues = `
    listen: "127.0.0.1:8080"
    loglevel: "info"
    concurrency: 0
    s3_timeout:  120s
    s3_retries:  10
    newrelic:
        name: "proto0-s3-helper"
        license: "None"
`
