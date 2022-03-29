package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"syscall"

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
		a.nrapp.RecordCustomMetric("s3-helper:s3error", float64(0))
		msg := fmt.Sprintf("[ERROR] s3:Get:Err - path:%s %v\n", s3Path, getErr)
		nrtxn.NoticeError(errors.New(msg))
		logger.Error().
			Str("error", getErr.Error()).
			Msg(fmt.Sprintf("s3:Get:Err - path:%s", s3Path))
		w.WriteHeader(500)
		return
	} else {
		defer getObject.Body.Close()
		a.nrapp.RecordCustomMetric("s3-helper:s3success", float64(0))
	}

	w.WriteHeader(200)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", *getObject.ContentLength))
	w.Header().Set("Content-Type", *getObject.ContentType)
	w.Header().Set("ETag", *getObject.ETag)

	bytes, err := io.Copy(w, getObject.Body)
	if err != nil {
		// we failed copying the body yet already sent the http header so can't tell
		// the client that it failed.
		if errors.Is(err, syscall.EPIPE) {
			a.nrapp.RecordCustomMetric("s3-helper:disconnect", float64(0))
			logger.Debug().
				Str("warning", err.Error()).
				Int64("content-length", *getObject.ContentLength).
				Int64("recv", bytes).
				Msg("nginx:bodywrite - client disconnect")
		} else {
			a.nrapp.RecordCustomMetric("s3-helper:failure", float64(0))
			msg := fmt.Sprintf("[ERROR] Nginx:Write:Err - path:%s %v\n", s3Path, err)
			nrtxn.NoticeError(errors.New(msg))
			logger.Error().
				Str("error", err.Error()).
				Int64("content-length", *getObject.ContentLength).
				Int64("recv", bytes).
				Msg("nginx:bodywrite - failure")
		}
	} else {
		a.nrapp.RecordCustomMetric("s3-helper:success", float64(0))
		logger.Debug().
			Str("path", s3Path).
			Int64("content-length", *getObject.ContentLength).
			Int64("recv", bytes).
			Msg("nginx:bodywrite - success")
	}
}
