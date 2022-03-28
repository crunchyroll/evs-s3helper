package main

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"

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
		logger.Error().
			Str("error", getErr.Error()).
			Msg(fmt.Sprintf("s3:Get:Err - path:%s ", s3Path))
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(200)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", *getObject.ContentLength))
	w.Header().Set("Content-Type", *getObject.ContentType)
	w.Header().Set("ETag", *getObject.ETag)

	io.Copy(w, getObject.Body)
	logger.Info().Str("path", s3Path).Int64("content-length", *getObject.ContentLength).Msg("s3:get - success")
}
