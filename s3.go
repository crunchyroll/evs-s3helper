package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"syscall"
	"time"

	awsauth "github.com/crunchyroll/go-aws-auth"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/rs/zerolog/log"
)

var statRate float32 = 1

// List of headers to forward in response
var headerForward = map[string]bool{
	"Content-Length": true,
	"Content-Range":  true,
	"Content-Type":   true,
	"Date":           true,
	"ETag":           true,
	"Last-Modified":  true,
}

const serverName = "VOD S3 Helper"

// Initialize process runtime
func initRuntime() {
	ncpus := runtime.NumCPU()
	log.Info().Msg(fmt.Sprintf("System has %d CPUs", ncpus))

	conc := ncpus
	if conf.Concurrency != 0 {
		conc = conf.Concurrency
	}
	log.Info().Msg(fmt.Sprintf("Setting thread concurrency to %d", conc))
	runtime.GOMAXPROCS(conc)
}

func (a *App) proxyS3Media(w http.ResponseWriter, r *http.Request) {
	nrtxn := a.nrapp.StartTransaction("S3Helper:proxyS3Media")
	defer nrtxn.End()
	w.Header().Set("Server", serverName)

	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}
	s3Path := r.URL.Path
	s3Bucket := conf.S3Bucket

	// Make sure that Remote Address is 127.0.0.1 so it comes off a local proxy
	addr := strings.SplitN(r.RemoteAddr, ":", 2)
	if len(addr) != 2 || addr[0] != "127.0.0.1" {
		w.WriteHeader(403)
		return
	}

	if r.URL.Path[:6] == "/avod/" {
		p := []rune(r.URL.Path)
		s3Path = string(p[5:])
		s3Bucket = conf.S3AdBucket
	}
	byterange := r.Header.Get("Range")
	logger := log.With().
		Str("object", s3Path).Str("range", byterange).Str("method", r.Method).Logger()

	// Bypass AWS SDK for S3 GetObject() call, sign and get the object manually via HTTP
	s3url := fmt.Sprintf("http://s3-%s.amazonaws.com/%s%s%s", conf.S3Region, s3Bucket, conf.S3Path, s3Path)
	log.Debug().Msg(fmt.Sprintf("Signed S3 URL: %s\n", s3url))
	r2, err := http.NewRequest(r.Method, s3url, nil)
	if err != nil {
		w.WriteHeader(403)
		logger.Error().
			Str("error", err.Error()).
			Str("url", s3url).
			Msg("Failed to create GET request to S3")
		return
	}

	r2 = awsauth.SignForRegion(r2, conf.S3Region, "s3")

	r2.Header.Set("Host", r2.URL.Host)
	// parse the byterange request header to derive the content-length requested
	// so we know how much data we need to xfer from s3 to the client.
	if byterange != "" {
		r2.Header.Set("Range", byterange)
	}

	var resp *http.Response

	// setup client outside of for loop since we don't
	// need to define it multiple times and failures
	// shouldn't need a new client
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 1 * time.Second,
			}).DialContext,
			IdleConnTimeout:   30 * time.Second,
			DisableKeepAlives: true, // terminates open connections
		}}

	resp, getErr := client.Do(r2)

	// resp is nil most likely if an error occurred
	if getErr != nil {
		// timeout error or network errors
		if netErr, ok := getErr.(net.Error); ok && netErr.Timeout() {
			// Timed out connecting to S3
			a.nrapp.RecordCustomMetric("s3-helper:timeout", float64(0))
			msg := fmt.Sprintf("AWS S3 Timeout for %s/%s", s3Bucket, s3Path)
			logger.Error().
				Str("error", netErr.Error()).
				Str("details", msg).
				Msg(fmt.Sprintf("s3:Get:Err - path:%s", s3Path))
		} else if netErr, ok := getErr.(net.Error); ok {
			// Network Error connecting to S3
			a.nrapp.RecordCustomMetric("s3-helper:neterror", float64(0))
			msg := fmt.Sprintf("AWS S3 Network Error for %s/%s", s3Bucket, s3Path)
			logger.Error().
				Str("error", netErr.Error()).
				Str("details", msg).
				Msg(fmt.Sprintf("s3:Get:Err - path:%s", s3Path))
		}
		// More network errors
		switch t := getErr.(type) {
		case *net.OpError:
			if t.Op == "dial" {
				// "Unknown host"
				a.nrapp.RecordCustomMetric("s3-helper:netunknownhost", float64(0))
				msg := fmt.Sprintf("AWS S3 Unknown Host Error for %s/%s", s3Bucket, s3Path)
				logger.Error().
					Str("error", getErr.Error()).
					Str("details", msg).
					Msg(fmt.Sprintf("s3:Get:Err - path:%s", s3Path))
			} else if t.Op == "read" {
				// "Connection refused"
				a.nrapp.RecordCustomMetric("s3-helper:connectionrefused", float64(0))
				msg := fmt.Sprintf("AWS S3 Connection Refused Error for %s/%s", s3Bucket, s3Path)
				logger.Error().
					Str("error", getErr.Error()).
					Str("details", msg).
					Msg(fmt.Sprintf("s3:Get:Err - path:%s", s3Path))
			}
		case syscall.Errno:
			if t == syscall.ECONNREFUSED {
				// "Connection refused"
				a.nrapp.RecordCustomMetric("s3-helper:connectionrefused", float64(0))
				msg := fmt.Sprintf("AWS S3 Connection Refused Error for %s/%s", s3Bucket, s3Path)
				logger.Error().
					Str("error", getErr.Error()).
					Str("details", msg).
					Msg(fmt.Sprintf("s3:Get:Err - path:%s", s3Path))
			}
		}

		// Casting to the awserr.Error type will allow you to inspect the error
		// code returned by the service in code. The error code can be used
		// to switch on context specific functionality. In this case a context
		// specific error message is printed to the user based on the bucket
		// and key existing.
		//
		// For information on other S3 API error codes see:
		// http://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html
		//
		// fmt.Println(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
		returnCode := 500 // return the actual error rather than generic 500
		msg := ""
		if aerr, ok := getErr.(awserr.Error); ok {
			if reqErr, ok := getErr.(awserr.RequestFailure); ok {
				returnCode = reqErr.StatusCode()
				if reqErr.StatusCode() == 503 {
					// AWS SlowDown Throttling S3 Bucket
					// Trick taken from: https://github.com/go-spatial/tegola/issues/458
					a.nrapp.RecordCustomMetric("s3-helper:s3slowdown503", float64(0))
					msg = fmt.Sprintf("SlowDown Throttling on %s/%s", s3Bucket, s3Path)
					logger.Error().
						Str("error", getErr.Error()).
						Str("details", msg).
						Msg(fmt.Sprintf("s3:Get:Err - path:%s", s3Path))
				} else if reqErr.StatusCode() == 404 {
					a.nrapp.RecordCustomMetric("s3-helper:s3status404", float64(0))
				} else {
					a.nrapp.RecordCustomMetric(fmt.Sprintf("s3-helper:s3status%d", reqErr.StatusCode()), float64(0))
				}
			}

			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				msg = fmt.Sprintf("bucket %s does not exist", s3Bucket)
				a.nrapp.RecordCustomMetric("s3-helper:s3nosuchbucket", float64(0))
			case s3.ErrCodeNoSuchKey:
				msg = fmt.Sprintf("object with key %s does not exist in bucket %s", s3Path, s3Bucket)
				a.nrapp.RecordCustomMetric("s3-helper:s3nosuchkey", float64(0))
			default:
				msg = fmt.Sprintf("s3 unknown error: %v %v %v", aerr.Code(), aerr.Message(), aerr.OrigErr())
				a.nrapp.RecordCustomMetric("s3-helper:s3unknownerror", float64(0))
			}
			logger.Error().
				Str("error", getErr.Error()).
				Str("details", msg).
				Msg(fmt.Sprintf("s3:Get:Err - path:%s", s3Path))
		}
		msg = fmt.Sprintf("[ERROR] s3:Get:Err - path:%s %v\n", s3Path, getErr)
		nrtxn.NoticeError(errors.New(msg))
		logger.Error().
			Str("error", getErr.Error()).
			Str("details", msg).
			Str("s3_statuscode: ", fmt.Sprintf("%d", returnCode)).
			Msg(fmt.Sprintf("s3:Get:Err - path:%s", s3Path))
		w.WriteHeader(500) // Return same error code back from S3 to Nginx
		return
	} else {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			a.nrapp.RecordCustomMetric("s3-helper:s3success", float64(0))
		} else {
			a.nrapp.RecordCustomMetric(fmt.Sprintf("s3-helper:s3badrespons%d", resp.StatusCode), float64(0))
			msg := fmt.Sprintf("[ERROR] s3:Get:Err - path:%s %v bad response code %d\n", s3Path, getErr, resp.StatusCode)
			w.WriteHeader(resp.StatusCode) // Return same error code back from S3 to Nginx
			nrtxn.NoticeError(errors.New(msg))
			logger.Error().
				Str("error", "Status code was outside 2xx range.").
				Str("http_statuscode", fmt.Sprintf("%d", resp.StatusCode)).
				Msg(fmt.Sprintf("s3:Get:Err - path:%s bad response code %d", s3Path, resp.StatusCode))
			return
		}
	}

	header := resp.Header
	for name, hflag := range headerForward {
		if hflag {
			if v := header.Get(name); v != "" {
				w.Header().Set(name, v)
			}
		}
	}

	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	w.Header().Set("Content-Type", resp.Header.Get("Content-type"))

	// Only return headers
	if r2.Method == "HEAD" {
		return
	}

	// Copy S3 body into buffer
	bytes, err := io.Copy(w, resp.Body)
	if err != nil {
		// we failed copying the body yet already sent the http header so can't tell
		// the client that it failed.
		if errors.Is(err, syscall.EPIPE) {
			a.nrapp.RecordCustomMetric("s3-helper:disconnect", float64(0))
			logger.Debug().
				Str("warning", err.Error()).
				Int64("content-length", resp.ContentLength).
				Int64("recv", bytes).
				Msg("s3:bodyread- s3 disconnect on body copy")
			return // S3 Disconnected during body copy
		} else {
			a.nrapp.RecordCustomMetric("s3-helper:failure", float64(0))
			msg := fmt.Sprintf("[ERROR] S3:Read:Err - path:%s %v\n", s3Path, err)
			nrtxn.NoticeError(errors.New(msg))
			logger.Error().
				Str("error", err.Error()).
				Int64("content-length", resp.ContentLength).
				Int64("recv", bytes).
				Msg("s3:bodyread- failure reading s3 body")
			return // S3 body copy Unknown Failure
		}
	} else {
		a.nrapp.RecordCustomMetric("s3-helper:success", float64(0))
		logger.Debug().
			Str("path", s3Path).
			Int64("content-length", resp.ContentLength).
			Int64("recv", bytes).
			Msg("nginx:bodywrite - success")
		return // Success
	}
}
