package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"syscall"

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
		s3Path = string(p[6:])
		s3Bucket = conf.S3AdBucket
	}
	byterange := r.Header.Get("Range")
	logger := log.With().
		Str("object", s3Path).Str("range", byterange).Str("method", r.Method).Logger()

	getObject, getErr := a.s3Client.GetObject(s3Bucket, s3Path, byterange)
	if getErr != nil {
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
			Msg(fmt.Sprintf("s3:Get:Err - path:%s", s3Path))
		w.WriteHeader(returnCode) // Return same error code back from S3 to Nginx
		return
	} else {
		defer getObject.Body.Close()
		a.nrapp.RecordCustomMetric("s3-helper:s3success", float64(0))
	}

	var buf *bytes.Buffer
	buf = new(bytes.Buffer)
	bytes, err := io.Copy(buf, getObject.Body)
	if err != nil {
		if errors.Is(err, syscall.EPIPE) {
			a.nrapp.RecordCustomMetric("s3-helper:disconnect", float64(0))
			logger.Debug().
				Str("warning", err.Error()).
				Int64("content-length", *getObject.ContentLength).
				Int64("recv", bytes).
				Msg("s3:bodyread- client disconnect")
			w.WriteHeader(500)
			return // Client Disconnected
		} else {
			a.nrapp.RecordCustomMetric("s3-helper:failure", float64(0))
			msg := fmt.Sprintf("[ERROR] S3:Read:Err - path:%s %v\n", s3Path, err)
			nrtxn.NoticeError(errors.New(msg))
			logger.Error().
				Str("error", err.Error()).
				Int64("content-length", *getObject.ContentLength).
				Int64("recv", bytes).
				Msg("s3:bodyread- failure")
			w.WriteHeader(500)
			return // Unknown Failure
		}
	} else {
		w.WriteHeader(200)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", *getObject.ContentLength))
		w.Header().Set("Content-Type", *getObject.ContentType)
		w.Header().Set("ETag", *getObject.ETag)
		bytes, err := io.Copy(w, buf)
		if err != nil {
			// we failed copying the body yet already sent the http header so can't tell
			// the client that it failed.
			if errors.Is(err, syscall.EPIPE) {
				a.nrapp.RecordCustomMetric("nginx:disconnect", float64(0))
				logger.Debug().
					Str("warning", err.Error()).
					Int64("content-length", *getObject.ContentLength).
					Int64("recv", bytes).
					Msg("nginx:bodywrite - client disconnect")
				return // Client Disconnected
			} else {
				a.nrapp.RecordCustomMetric("nginx:failure", float64(0))
				msg := fmt.Sprintf("[ERROR] Nginx:Write:Err - path:%s %v\n", s3Path, err)
				nrtxn.NoticeError(errors.New(msg))
				logger.Error().
					Str("error", err.Error()).
					Int64("content-length", *getObject.ContentLength).
					Int64("recv", bytes).
					Msg("nginx:bodywrite - failure")
				return // Unknown Failure
			}
		}

		a.nrapp.RecordCustomMetric("s3-helper:success", float64(0))
		logger.Debug().
			Str("path", s3Path).
			Int64("content-length", *getObject.ContentLength).
			Int64("recv", bytes).
			Msg("nginx:bodywrite - success")
		return // Success
	}
}
