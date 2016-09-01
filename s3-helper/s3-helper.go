// s3-helper is used to assist nginx with various AWS related tasks
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/crunchyroll/evs-common/config"
	"github.com/crunchyroll/evs-common/logging"
	"github.com/crunchyroll/evs-common/util"
	"github.com/crunchyroll/evs-s3helper/mapper"
	"github.com/crunchyroll/go-aws-auth"
)

const mediaRoot = "/media"

// Default config file
const configFileDefault = "/mob/etc/s3-helper.yml"

// Config holds the global config
type Config struct {
	Listen  string `yaml:"listen"`
	Logging logging.Config

	Concurrency int `optional:"true"`

	S3Region string `yaml:"s3_region"`
	S3Bucket string `yaml:"s3_bucket"`
	S3Path   string `yaml:"s3_prefix" optional:"true"`

	Map mapper.Config `yaml:"map" optional:"true"`

	StatsdAddr        string `yaml:"statsd_addr"`
	StatsdEnvironment string `yaml:"statsd_env"`
}

const defaultConfValues = `
    listen: "127.0.0.1:8080"
    logging:
        ident: "s3-helper"
        level: "info"
    concurrency:   0
    statsd_addr:   "127.0.0.1:8125"
    statsd_env:    dev
`

var conf Config
var progName string
var statRate float32 = 1
var statter statsd.Statter

// List of swift headers to forward in response
var headerForward = map[string]bool{
	"Date":           true,
	"Content-Length": true,
	"Content-Range":  true,
	"Content-Type":   true,
	"Last-Modified":  true,
	"ETag":           true,
}

const serverName = "Ellation VOD Server"

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

func initStatsd() {
	var err error

	if conf.StatsdAddr == "" {
		statter, err = statsd.NewNoop()
	} else {
		prefix := fmt.Sprintf("%s.%s", conf.StatsdEnvironment, progName)
		statter, err = statsd.New(conf.StatsdAddr, prefix)
	}

	util.SetStatterForTimer(statter)

	if err != nil {
		logging.Panicf("Couldn't initialize statsd: %v", err)
	}
}

func forwardToS3(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}

	logging.Debugf("request for media object: %s", r.URL.String())

	// Make sure that RemoteAddr is 127.0.0.1 so it comes off a local proxy
	a := strings.SplitN(r.RemoteAddr, ":", 2)
	if len(a) != 2 || a[0] != "127.0.0.1" {
		w.WriteHeader(403)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, mediaRoot)
	s3url := fmt.Sprintf("http://s3-%s.amazonaws.com/%s%s%s", conf.S3Region, conf.S3Bucket, conf.S3Path, path)
	r2, err := http.NewRequest(r.Method, s3url, nil)
	if err != nil {
		statter.Inc("s3_serve.fail.req_creation", 1, 1)
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

	client := &http.Client{}
	resp, err := client.Do(r2)
	if err != nil {
		logging.Errorf("S3 error: %v", err)
		w.WriteHeader(500)
		return
	}

	defer resp.Body.Close()

	w.Header().Set("Server", serverName)

	header := resp.Header
	for name, flag := range headerForward {
		if flag {
			if v := header.Get(name); v != "" {
				w.Header().Set(name, v)
			}
		}
	}

	logging.Debugf("S3 transfer %s [%s]", resp.Status, url)

	w.WriteHeader(resp.StatusCode)
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		if r2.Method != "HEAD" {
			bytes, err := io.Copy(w, resp.Body)
			if err != nil {
				logging.Infof("Failed to copy response for %s - %v (TCP disconnect?)", url, err)
			} else {
				statter.Inc("s3_serve.bytes", bytes, 1)
				logging.Debugf("S3 transfered %d bytes from %v", bytes, url)
			}
		}
	}
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	logging.Debugf("404: not found: %s", r.URL.String())
	w.WriteHeader(404)
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
	initStatsd()

	statter.Inc("start", 1, 1)
	defer statter.Inc("stop", 1, 1)

	m := mapper.NewMapper(&conf.Map, statter)

	mux := http.NewServeMux()

	mux.Handle(mediaRoot+"/", http.HandlerFunc(forwardToS3))
	mux.Handle("/map/", http.HandlerFunc(m.MapManifest))
	mux.Handle("/", http.HandlerFunc(notFoundHandler))

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
