package controllers

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// SystemController - the struct to contain all key information for serving the system routes
type SystemController struct {
	controller
	semVer []byte
}

// NewSystemController - instantiates a SystemController instance
func NewSystemController(buildPath string) (*SystemController, error) {
	buildInfo, err := ioutil.ReadFile(buildPath)
	if err != nil {
		return nil, fmt.Errorf("Error in instantiating a new system controller. Error: %+v", err.Error())
	}
	return &SystemController{controller: &BaseController{}, semVer: buildInfo}, nil
}

// SystemHealth  returns a heart-beat response with status 200
func (sc *SystemController) SystemHealth(resp http.ResponseWriter, req *http.Request) {
	sc.SuccessEmpty(resp)
}

// SystemBuild  returns the content of BUILD_INFO (semantic version) with status 200
func (sc *SystemController) SystemBuild(resp http.ResponseWriter, req *http.Request) {
	err := sc.Success(resp, req, &sc.semVer)
	if err != nil {
		fmt.Printf("Error calling 'Success' method for system controller")
		sc.InternalServerError(resp)
	}
}
