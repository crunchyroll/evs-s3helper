package main

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/crunchyroll/evs-s3helper/awsclient"
	newrelic "github.com/newrelic/go-agent/v3/newrelic"
	"github.com/rs/zerolog/log"
)

// PORT - The default port no, used no config doesn't have a port no defined.
const PORT = 3300

// App - a struct to hold the entire application context
type App struct {
	router   *http.ServeMux
	s3Client *awsclient.S3Client
	nrapp    *newrelic.Application
}

// Initialize - start the app with a path to config yaml
func (a *App) Initialize(pprofFlag *bool, s3Region string) {
	s3Client, err := awsclient.NewS3Client(s3Region)
	if err != nil {
		fmt.Printf("App failed to initiate due to invalid S3 client. error: %+v\n", err)
		os.Exit(1) // kill the app
	}

	a.s3Client = s3Client
	a.router = http.NewServeMux()

	nrapp, nrerr := newrelic.NewApplication(
		newrelic.ConfigAppName(conf.NewRelic.Name),
		newrelic.ConfigLicense(conf.NewRelic.License),
		newrelic.ConfigInfoLogger(os.Stdout),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	if nil != nrerr {
		fmt.Println(nrerr)
		panic(fmt.Sprintf("newrelic is not configured. error: %+v.", nrerr))
	}
	a.nrapp = nrapp

	initRuntime()

        // Use /avod/ for Ads config bucket vs. / for content bucket
	a.router.Handle("/", http.HandlerFunc(a.proxyS3Media))

	if *pprofFlag {
		a.router.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		a.router.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		a.router.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		a.router.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		log.Info().Msg("pprof is enabled")
	}

	log.Info().Msg(fmt.Sprintf("Accepting connections on %v", conf.Listen))
	return
}

// Run - run the application with loaded App struct
func (a *App) Run(port string) {
	fmt.Printf("App start up initiated.\n")
	errLNS := http.ListenAndServe(port, a.router)
	defer fmt.Print("App shutting down")

	if errLNS != nil {
		fmt.Printf("App failed to start up. Error: %+v\n", errLNS)
		os.Exit(1)
	}
}

func (a *App) initializeRoutes() {
}
