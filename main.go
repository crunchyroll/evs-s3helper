package main

import (
	"flag"
	"fmt"
	"math/rand"
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
	log.Info().Msg(fmt.Sprintf("Loaded config from %s", *configFile))

	if conf.Logging.Level == "error" {
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	} else if conf.Logging.Level == "warn" {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	} else if conf.Logging.Level == "info" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else if conf.Logging.Level == "debug" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if conf.Logging.Level == "panic" {
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	} else if conf.Logging.Level == "fatal" {
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
		log.Error().Msg(fmt.Sprintf("Bad loglevel given %s - defaulting to Warn level", conf.Logging.Level))
	}

	api := App{}
	api.Initialize(pprofFlag, conf.S3Region)
	api.Run(conf.Listen)

	stopSignals := make(chan os.Signal, 1)
	signal.Notify(stopSignals, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)
	<-stopSignals
}
