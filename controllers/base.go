package controllers

import (
	"net/http"
)

// BaseController - the struct to hold base controller
type BaseController struct {
	controller
}

type controller interface {
	BadGateway(http.ResponseWriter)
	Failure(w http.ResponseWriter, errorCode int, errorMessage string)
	Forbidden(http.ResponseWriter)
	InternalServerError(http.ResponseWriter)
	ServiceUnavailable(http.ResponseWriter)
	Success(http.ResponseWriter, *http.Request, *[]byte) error
	SuccessEmpty(http.ResponseWriter)
	Unauthorized(http.ResponseWriter)
}

// Success - base method for sending HTTP 200
func (c *BaseController) Success(w http.ResponseWriter, r *http.Request, data *[]byte) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(*data)
	return err
}

// SuccessEmpty - base method for sending HTTP 200 (without content)
func (c *BaseController) SuccessEmpty(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
}

// ServiceUnavailable - base method for sending HTTP 503
func (c *BaseController) ServiceUnavailable(w http.ResponseWriter) {
	w.WriteHeader(http.StatusServiceUnavailable)
}

// Failure - base method for sending HTTP 503 with additional params
func (c *BaseController) Failure(w http.ResponseWriter, errorCode int, errorMessage string) {
	// TODO: check if the error maps to any of the http status codes
	// if not, bubble up error Code using the fail-over mechanism

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
}

// InternalServerError - base method for sending HTTP 500
func (c *BaseController) InternalServerError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
}

// BadGateway - base method for sending HTTP 502
func (c *BaseController) BadGateway(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadGateway)
}

// Unauthorized - base method for sending HTTP 401
func (c *BaseController) Unauthorized(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
}

// Forbidden - base method for sending HTTP 403
func (c *BaseController) Forbidden(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
}
