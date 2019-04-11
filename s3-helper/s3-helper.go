// s3-helper is used to assist nginx with various AWS related tasks
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/crunchyroll/evs-common/config"
	"github.com/crunchyroll/evs-common/logging"
	"github.com/crunchyroll/evs-common/newrelic"
	"github.com/crunchyroll/go-aws-auth"
)

// Default config file
const configFileDefault = "/etc/s3-helper.yml"

// Config holds the global config
type Config struct {
	Listen  string `yaml:"listen"`
	Logging logging.Config

	Concurrency int `optional:"true"`

	S3Timeout time.Duration `yaml:"s3_timeout"`
	S3Retries int           `yaml:"s3_retries"`

	S3Region string `yaml:"s3_region"`
	S3Bucket string `yaml:"s3_bucket"`
	S3Path   string `yaml:"s3_prefix" optional:"true"`

	// Keep the NewRelic as optional, so we don't remove it from ellation_formation
	NewRelic          newrelic.Config `yaml:"newrelic" optional:"true"`
	StatsdAddr        string          `yaml:"statsd_addr"`
	StatsdEnvironment string          `yaml:"statsd_env"`
}

const defaultConfValues = `
    listen: "127.0.0.1:8080"
    logging:
        ident: "s3-helper"
        level: "info"
    newrelic:
        name:    ""
        license: ""
    s3_timeout:  5s
    s3_retries:  5
    concurrency:   0
    statsd_addr:   ""
    statsd_env:    ""
`

var conf Config
var progName string
var statRate float32 = 1

// List of headers to forward in response
var headerForward = map[string]bool{
	"Date":           true,
	"Content-Length": true,
	"Content-Range":  true,
	"Content-Type":   true,
	"Last-Modified":  true,
	"ETag":           true,
}

const serverName = "VOD S3 Helper"

// Initialize process runtime
func initRuntime() {
	ncpus := runtime.NumCPU()
	logging.Infof("System has %d CPUs", ncpus)

	conc := ncpus
	if conf.Concurrency != 0 {
		conc = conf.Concurrency
	}
	logging.Infof("Setting thread concurrency to %d", conc)
	runtime.GOMAXPROCS(conc)
}

func forwardToS3(w http.ResponseWriter, r *http.Request) {
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

	path := r.URL.Path
	s3url := fmt.Sprintf("http://s3-%s.amazonaws.com/%s%s%s", conf.S3Region, conf.S3Bucket, conf.S3Path, path)
	r2, err := http.NewRequest(r.Method, s3url, nil)
	if err != nil {
		logging.Errorf("Failed to create GET request for S3 object - %v", err)
		return
	}

	r2 = awsauth.SignForRegion(r2, conf.S3Region, "s3")

	url := r2.URL.String()

	logging.Debugf("Forwarding request to %s", url)

	r2.Header.Set("Host", r2.URL.Host)
	if byterange := r.Header.Get("Range"); byterange != "" {
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
			IdleConnTimeout: conf.S3Timeout,
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
			logging.Errorf("S3 connection failed after #%d retries: %v", conf.S3Retries, err)
			w.WriteHeader(500)
			return
		}

		logging.Errorf("S3 connection timeout, retry #%d: %s", nretries, s3url)
		nretries++
	}

	defer resp.Body.Close()

	header := resp.Header
	for name, flag := range headerForward {
		if flag {
			if v := header.Get(name); v != "" {
				w.Header().Set(name, v)
			}
		}
	}
	// set the content length so failures are caught by client
	//w.Header().Set("Content-Length", fmt.Sprintf("%d", resp.Body.ContentLength))

	logging.Debugf("S3 transfer %s [%s]", resp.Status, url)

	// we can't buffer in ram or to disk so write the body
	// directly to the return body buffer and stream out
	// to the client. if we have a failure, we can't notify
	// the client, this is a poor design with potential
	// silent truncation of the output.
	w.WriteHeader(resp.StatusCode)
	var bytes int64
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		if r2.Method != "HEAD" {
			nretries = 0
			for {
				nretries++
				var nbytes int64
				nbytes, err = io.Copy(w, resp.Body)
				// retry 3 times, copy continues
				// where it left off each round.
				bytes += nbytes
				if err == nil || nretries >= 3 {
					// too many retries or success
                                        // force the body to close when we fully fail
					resp.Body.Close()
					break
				} else {
					logging.Errorf("S3 failed to copy response for %s (try #%d %d bytes / %d bytes) - %v",
						url, nretries, nbytes, bytes, err)
				}
			}
			if err != nil {
				// we failed copying the body yet already sent the http header so can't tell
				// the client that it failed.
				logging.Errorf("S3 failed to copy response for %s (%d bytes) - %v", url, bytes, err)
			} else {
				logging.Debugf("S3 transfered %d bytes from %v", bytes, url)
			}
		}
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	progName = path.Base(os.Args[0])

	configFile := flag.String("config", configFileDefault, "config file to use")
	pprofFlag := flag.Bool("pprof", false, "enable pprof")
	flag.Parse()

	if !config.Load(*configFile, defaultConfValues, &conf) {
		log.Printf("Unable to load config from %s - terminating", *configFile)
		return
	}

	logging.Init(&conf.Logging)
	logging.Infof("%s starting up", progName)
	defer logging.Infof("%s shutting down", progName)

	logging.Infof("Loaded config from %v", *configFile)

	initRuntime()

	// nr := newrelic.NewNewRelic(&conf.NewRelic)
	mux := http.NewServeMux()

	// mux.Handle(nr.MonitorHandler("/", http.HandlerFunc(forwardToS3)))
	mux.Handle("/", http.HandlerFunc(forwardToS3))

	if *pprofFlag {
		mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		logging.Infof("pprof is enabled")
	}

	logging.Infof("Accepting connections on %v", conf.Listen)

	go func() {
		logging.Panicf("%v", http.ListenAndServe(conf.Listen, mux))
	}()

	stopSignals := make(chan os.Signal, 1)
	signal.Notify(stopSignals, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)
	<-stopSignals
}
