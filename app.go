package main

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/crunchyroll/evs-s3helper/awsclient"
	"github.com/rs/zerolog/log"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	nrgorilla "github.com/newrelic/go-agent/v3/integrations/nrgorilla"
	newrelic "github.com/newrelic/go-agent/v3/newrelic"
)

// PORT - The default port no, used no config doesn't have a port no defined.
const PORT = 3300

// App - a struct to hold the entire application context
type App struct {
	router   *mux.Router
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

	a.s3Client = s3Client
	a.router = mux.NewRouter()

	a.router.Use(nrgorilla.Middleware(nrapp))

	initRuntime()

	a.router.HandleFunc("/avod", a.forwardToS3ForAd)
	a.router.HandleFunc("/", forwardToS3ForMedia)

	if *pprofFlag {
		a.router.HandleFunc("/debug/pprof/", pprof.Index)
		a.router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		a.router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		a.router.HandleFunc("/debug/pprof/profile", pprof.Profile)
		log.Info().Msg("pprof is enabled")
	}

	log.Info().Msg(fmt.Sprintf("Accepting connections on %v", conf.Listen))
	return
}

// Run - run the application with loaded App struct
func (a *App) Run(port string) {
	fmt.Printf("App start up initiated.\n")
	s := &http.Server{
		Addr:           port,
		Handler:        handlers.CORS()(nrgorilla.InstrumentRoutes(a.router, a.nrapp)),
		ReadTimeout:    120 * time.Second,
		WriteTimeout:   120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	errLNS := s.ListenAndServe()
	if errLNS != nil {
		fmt.Printf("App failed to start up. Error: %+v\n", errLNS)
		os.Exit(1)
	}
}

func (a *App) initializeRoutes() {
}
