package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/crunchyroll/evs-common/config"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Default config file
const configFileDefault = "/etc/s3-helper.yml"

var conf Config
var progName string

func main() {
	zerolog.TimeFieldFormat = ""
	rand.Seed(time.Now().UnixNano())

	progName = path.Base(os.Args[0])

	configFile := flag.String("config", configFileDefault, "config file to use")
	pprofFlag := flag.Bool("pprof", false, "enable pprof")
	flag.Parse()

	if !config.Load(*configFile, defaultConfValues, &conf) {
		log.Error().Msg(fmt.Sprintf("Unable to load config from %s - terminating", *configFile))
		return
	}
	if conf.LogLevel == "error" {
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	} else if conf.LogLevel == "warn" {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	} else if conf.LogLevel == "info" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else if conf.LogLevel == "debug" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if conf.LogLevel == "panic" {
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	} else if conf.LogLevel == "fatal" {
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
		log.Error().Msg(fmt.Sprintf("Bad loglevel given %s - defaulting to Warn level", conf.LogLevel))
	}

	log.Info().Msg("Starting up")
	defer log.Info().Msg("Shutting down")

	log.Info().Msg(fmt.Sprintf("Loaded config from %s", *configFile))

	initRuntime()

	mux := http.NewServeMux()

	mux.Handle("/avod/", http.HandlerFunc(forwardToS3ForAd))
	mux.Handle("/", http.HandlerFunc(forwardToS3ForMedia))

	if *pprofFlag {
		mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		log.Info().Msg("pprof is enabled")
	}

	log.Info().Msg(fmt.Sprintf("Accepting connections on %v", conf.Listen))

	go func() {
		errLNS := http.ListenAndServe(conf.Listen, mux)
		if errLNS != nil {
			log.Error().Msg(fmt.Sprintf("Failure starting up %v", errLNS))
			os.Exit(1)
		}
	}()

	stopSignals := make(chan os.Signal, 1)
	signal.Notify(stopSignals, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)
	<-stopSignals
}
