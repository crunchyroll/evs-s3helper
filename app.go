package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	newrelic "github.com/newrelic/go-agent"
	nrgorilla "github.com/newrelic/go-agent/_integrations/nrgorilla/v1"

	"github.com/crunchyroll/evs-s3helper/config"
	"github.com/crunchyroll/evs-s3helper/controllers"
	"github.com/crunchyroll/evs-s3helper/logging"
)

// PORT - The default port no, used config doesn't have a port no defined.
const PORT = 8800

// App - a struct to hold the entire application context
type App struct {
	Configs  *config.AppConfig
	Logger   *logging.Logger
	Newrelic newrelic.Application
	Router   *mux.Router
}

// Initialize - start the app with a path to config yaml
func (a *App) Initialize(configpath string) {
	var err error

	// load the configs
	a.Configs, err = config.LoadConfiguration(configpath)
	if err != nil {
		fmt.Printf("s3-proxy failed to start due to invalid config. error: %+v, config-path: %s", err.Error(), configpath)
		os.Exit(1) // kill the app
	}

	// initializing logging
	logConfig := logging.LogConfig{AppName: a.Configs.Logging.AppName, AppVersion: a.Configs.Logging.AppVersion, EngGroup: a.Configs.Logging.EngGroup, Level: a.Configs.Logging.Level}
	logger, err := logging.New(&logConfig)
	if err != nil {
		fmt.Printf("s3-helper failed to init logger. Error: %+v", err.Error())
		os.Exit(1) // kill the app
	}

	// attach the logger instrument to the app struct
	a.Logger = &logger

	a.Logger.Debug("enable monitoring via Newrelic")
	cfg := newrelic.NewConfig(a.Configs.Newrelic.Name, a.Configs.Newrelic.License)

	// ignore HTTP 400, HTTP 401 & HTTP 403 on Newrelic
	cfg.ErrorCollector.IgnoreStatusCodes = append(cfg.ErrorCollector.IgnoreStatusCodes, http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden)
	a.Newrelic, err = newrelic.NewApplication(cfg)
	if err != nil {
		a.Logger.Error("Couldn't start the Newrelic agent", logging.DataFields{"error": err.Error()})
	}

	logger.Debug("enabling router")
	a.Router = mux.NewRouter()

	// register routes
	a.initializeRoutes()
}

// Run - run the application with loaded App struct
func (a *App) Run() {
	addr := fmt.Sprintf(":%d", a.getPort())

	a.Logger.Info("starting S3 Helper Service", logging.DataFields{"port": addr})
	err := http.ListenAndServe(addr, nrgorilla.InstrumentRoutes(a.Router, a.Newrelic))
	if err != nil {
		a.Logger.Error("An error occurred starting the server. Going to call exit", logging.DataFields{"Error": err.Error()})
		os.Exit(1)
	}
}

func (a *App) initializeRoutes() {
	a.Logger.Debug("registering routes and preparing controllers")

	// init controllers
	sc, _ := controllers.NewSystemController("BUILD_INFO")

	// ELB specific health check endpoint
	a.Router.HandleFunc("/_health", sc.SystemHealth).Methods("GET")
	a.Logger.Debug("route: `/_health` with `GET` method initiated")
}

// getPort - returns the port number from the config. If no port no
// is defined in the config, returns a default (PORT)
func (a *App) getPort() int {
	if a.Configs.Service.Listen == 0 {
		a.Logger.Debug("no port specified, using default port", logging.DataFields{"port": PORT})
		return PORT
	}
	return a.Configs.Service.Listen
}
