## Overview

s3helper signs S3 object requests using instance credentials.  It only accepts connections from 127.0.0.1
and only accepts GET and HEAD methods.  It provides no crossdomain.xml (though this can be put in the S3
bucket).

## Building

Clone this repo, then:

```
make setup
make build
```

## Arguments

Create a config in `s3-helper.yml`.  Start the service with:

```
$ ./s3-helper -config=s3-helper.yml
```

Run "s3-helper -h" which list possible flags.

## Usage

```
\\ for  basic get request (goes to the default media bucket)
127.0.0.1/{bucket-name}/{object-name}

\\ for get request with byte range
curl -H "range: bytes=0-199" 127.0.0.1/{bucket-name}/{object-name}

\\ basic get request (goes to the ad media bucket)
127.0.0.1/avod/{bucket-name}/{object-name}

\\ for get request with byte range
curl -H "range: bytes=0-199" 127.0.0.1/avod/{bucket-name}/{object-name}
```
## Configuration

s3helper reads its configuration from a file in yml format.  The default location is /etc/s3-helper.yml,
but this can be changed with the -config option, e.g. "-config=./test.yml"

```yml
    listen: <endpoint, default is ":8080">
    logging:
            ident: <syslog ident, default is "s3-helper">
            level: <syslog level, default is "info">
    concurrency: <explicit runtime concurrency, default is 0 which makes it match # of CPUs>
    statsd_addr:  <default is "127.0.0.1:8125">
    statsd_env:   <default is "dev">
    newrelic:
        name:    <newrelic name, default is "">
        license: <newrelic license, default is "">

    s3_bucket:  <name of S3 bucket to forward object requests to>
    s3_region:  <region of S3 bucket>
    s3_path:    <optional prefix to prepend to object requests>
    s3_retries; <maximum number of S3 retries>
    s3_timeout: <timeout for S3 requests>
```

## Behavior

Assume the configuration consists of:

    s3_bucket:  SomeBucket
    s3_region:  SomeRegion
    s3_path:    /SomePath
    s3_timeout: 3s
    s3_retries: 3

s3helper receives an HTTP request from 127.0.0.1, e.g. `GET /SomeDirectory/manifest.json`
It takes this request and maps it to an S3 bucket URL,
    `http://s3-SomeRegion.amazonaws.com/SomeBucket/SomePath/SomeDirectory/manifest.json`
This request is signed using the EC2 instance credentials for its first AMI role.
An http GET request for this is made.
The result is forwarded and the following headers retained:
    "Date"
    "Content-Length"
    "Content-Range"
    "Content-Type"
    "Last-Modified"
    "ETag"

Range requests are fully supported.  As a note, Range requests produce 206 responses from S3,
and these are faithfully forwarded.

Any amazon specific headers are removed.

Setting s3_timeout causes requests to fail after a specific time.  We've found a very small number
of S3 requests will take an extraordinary long time for a response and simply retrying them yields a
prompt response.  s3_retries sets the number of timeout retries (other errors are not currently
retried).

This permits e.g. use of nginx in front of s3helper without nginx having to know a single thing
about S3, credentials, or magic headers.


## Statsd

s3helper outputs stats for object retrieval times and request counts to the configured statsd/collectd
endpoint.  If the address and env are set to "" (the default) no stats are emitted.


## New Relic

Basic request metrics are forwarded to New Relic if either the license or name is non-empty.  (Default is
"", meaning nothing is forwarded.)  Requires running the nr-agent on the host and an NR account.  It's what
we mainly rely on here at Ellation.


## License

Released under the MIT License.  See LICENSE.md
