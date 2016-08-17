## Overview

s3helper signs S3 object requests using instance credentials.  It only accepts connections from 127.0.0.1
and only accepts GET and HEAD methods.  It provides no crossdomain.xml (though this can be put in the S3
bucket).


## Arguments

Run "playback-api -h" to see a list of arguments.


## Configuration

s3helper reads its configuration from a file in yml format.  The default location is /mob/etc/s3-helper.yml,
but this can be changed with the -config option, e.g. "-config=./test.yml"

**Top-level config**

    listen: <endpoint, default is ":8080">
    logging:
            ident: <syslog ident, default is "s3-helper">
            level: <syslog level, default is "info">
                concurrency: <explicit runtime concurrency, default is 0 which makes it match # of CPUs>

    statsd_addr:  <default is "127.0.0.1:8125">
    statsd_environment: <default is "dev">
    
    s3_bucket:  <name of S3 bucket to forward object requests to>
    s3_region:  <region of S3 bucket>
    s3_path:    <optional prefix to prepend to object requests>
    
    
### Behavior

Assume the configuration consists of:

    s3_bucket:  evs-dev
    s3_region:  us-west-2
    s3_path:    /chris

s3helper receives an HTTP request from 127.0.0.1, e.g. `GET /abcdef12345678/manifest.json`
It takes this requests and maps it to an S3 bucket URL,
    `http://s3-us-west-2.amazonaws.com/evs-dev/chris/abcdef12345678/manifest.json`
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

This permits e.g. use of nginx in front of s3helper without nginx having to know a single thing
about S3, credentials, or magic headers.
