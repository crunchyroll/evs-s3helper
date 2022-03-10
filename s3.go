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

func (a *App) forwardToS3ForAd(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", serverName)

	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}

	// Make sure that Remote Address is 127.0.0.1 so it comes off a local proxy
	addr := strings.SplitN(r.RemoteAddr, ":", 2)
	if len(addr) != 2 || addr[0] != "127.0.0.1" {
		w.WriteHeader(403)
		return
	}

	p := []rune(r.URL.Path)
	s3Path := string(p[6:])
	byterange := r.Header.Get("Range")
	logger := log.With().
		Str("object", s3Path).Str("range", byterange).Str("method", r.Method).Logger()

	getObject, getErr := a.s3Client.GetObject(conf.S3AdBucket, s3Path, byterange)
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
	logger.Debug().Str("path", s3Path).Int64("content-length", *getObject.ContentLength).Msg("s3:get - success")
}

func forwardToS3ForMedia(w http.ResponseWriter, r *http.Request) {
	forwardToS3(w, r, conf.S3Bucket)
}

func forwardToS3(w http.ResponseWriter, r *http.Request, bucket string) {
	w.Header().Set("Server", serverName)

	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}

	// Make sure that RemoteAddr is 127.0.0.1 so it comes off a local proxy
	a := strings.SplitN(r.RemoteAddr, ":", 2)
	if len(a) != 2 || a[0] != "127.0.0.1" {
		w.WriteHeader(403)
		return
	}

	upath := r.URL.Path
	byterange := r.Header.Get("Range")
	logger := log.With().
		Str("object", upath).
		Str("range", byterange).
		Str("method", r.Method).
		Logger()
	s3url := fmt.Sprintf("http://s3-%s.amazonaws.com/%s%s%s", conf.S3Region, bucket, conf.S3Path, upath)
	r2, err := http.NewRequest(r.Method, s3url, nil)
	if err != nil {
		w.WriteHeader(403)
		logger.Error().
			Str("error", err.Error()).
			Str("url", s3url).
			Msg("Failed to create GET request")
		return
	}

	r2 = awsauth.SignForRegion(r2, conf.S3Region, "s3")

	url := r2.URL.String()
	logger.Debug().
		Str("url", url).
		Msg("Received request")

	var bodySize int64
	r2.Header.Set("Host", r2.URL.Host)
	// parse the byterange request header to derive the content-length requested
	// so we know how much data we need to xfer from s3 to the client.
	if byterange != "" {
		r2.Header.Set("Range", byterange)
	}

	nretries := 0

	var resp *http.Response

	// setup client outside of for loop since we don't
	// need to define it multiple times and failures
	// shouldn't need a new client
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   conf.S3Timeout,
				KeepAlive: 1 * time.Second,
			}).DialContext,
			IdleConnTimeout:   conf.S3Timeout,
			DisableKeepAlives: true, // terminates open connections
		}}

	for {
		resp, err = client.Do(r2)
		if err == nil {
			break
		}

		// Bail out on non-timeout error, or too many timeouts.
		netErr, ok := err.(net.Error)
		isTimeout := ok && netErr.Timeout()

		if nretries >= conf.S3Retries || !isTimeout {
			logger.Error().
				Str("error", err.Error()).
				Msg(fmt.Sprintf("Connection failed after #%d retries", conf.S3Retries))
			w.WriteHeader(500)
			return
		}

		logger.Error().
			Str("error", err.Error()).
			Msg(fmt.Sprintf("Connection timeout: retry #%d", nretries))
		nretries++
	}

	defer resp.Body.Close()

	header := resp.Header
	for name, hflag := range headerForward {
		if hflag {
			if v := header.Get(name); v != "" {
				w.Header().Set(name, v)
			}
		}
	}

	// we can't buffer in ram or to disk so write the body
	// directly to the return body buffer and stream out
	// to the client. if we have a failure, we can't notify
	// the client, this is a poor design with potential
	// silent truncation of the output.
	w.WriteHeader(resp.StatusCode)
	var bytes int64
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		if r2.Method != "HEAD" {
			logger.Debug().
				Int64("content-length", bodySize).
				Msg(fmt.Sprintf("Begin data transfer of #%d bytes", bodySize))
			bytes, err = io.Copy(w, resp.Body)
			if err != nil {
				// we failed copying the body yet already sent the http header so can't tell
				// the client that it failed.
				if errors.Is(err, syscall.EPIPE) {
					logger.Debug().
						Str("warning", err.Error()).
						Int64("content-length", bodySize).
						Int64("recv", bytes).
						Msg("Client Disconnected, copy body truncated.")
				} else {
					logger.Error().
						Str("error", err.Error()).
						Int64("content-length", bodySize).
						Int64("recv", bytes).
						Msg("Failed to copy body to client, giving up.")
				}
			} else {
				logger.Debug().
					Int64("content-length", bodySize).
					Int64("recv", bytes).
					Msg("Success copying body")
			}
		}
	} else {
		logger.Error().
			Str("error", fmt.Sprintf("Response Status Code: %d", resp.StatusCode)).
			Int("statuscode", resp.StatusCode).
			Int64("content-length", bodySize).
			Int64("recv", bytes).
			Msg("Bad connection status response code")
	}
}
