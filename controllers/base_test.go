package controllers_test

import (
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/crunchyroll/evs-s3helper/controllers"
)

var _ = Describe("BaseController", func() {
	ctrl := new(BaseController)
	var w *httptest.ResponseRecorder

	Context("when any method for the baseController is called", func() {
		BeforeEach(func() {
			w = httptest.NewRecorder()
		})

		It("returns a HTTP 200 when 'SuccessEmpty' is called.", func() {
			ctrl.SuccessEmpty(w)
			Expect(w.Code).To(Equal(200))
		})

		It("returns a HTTP 503 when 'Failure' is called.", func() {
			ctrl.Failure(w, 123, "some specific error")
			Expect(w.Code).To(Equal(503))
		})

		It("returns a HTTP 500 when 'InternalServerError' is called.", func() {
			ctrl.InternalServerError(w)
			Expect(w.Code).To(Equal(500))
		})

		It("returns a HTTP 502 when 'BadGateway' is called.", func() {
			ctrl.BadGateway(w)
			Expect(w.Code).To(Equal(502))
		})

		It("returns a HTTP 401 when 'Unauthorized' is called.", func() {
			ctrl.Unauthorized(w)
			Expect(w.Code).To(Equal(401))
		})

		It("returns a HTTP 503 when 'ServiceUnavailable' is called.", func() {
			ctrl.ServiceUnavailable(w)
			Expect(w.Code).To(Equal(503))
		})

		It("returns a HTTP 403 when 'Forbidden' is called.", func() {
			ctrl.Forbidden(w)
			Expect(w.Code).To(Equal(403))
		})
	})
})
