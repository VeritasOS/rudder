package resource

import (
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/emicklei/go-restful"
)

var (
	errFailToReadResponse  = restful.NewError(http.StatusBadRequest, "unable to read request body")
	errFailToWriteResponse = restful.NewError(http.StatusInternalServerError, "unable to write response")
)

// errorResponse creates an error response from the given error
func errorResponse(res *restful.Response, err restful.ServiceError) {
	log.WithError(err).Error(err.Message)
	if err := res.WriteServiceError(err.Code, err); err != nil {
		log.WithError(err).Error("unable to write error")
	}
}

func errorResponsetest(res *restful.Response, err error) {
	log.WithError(err).Error(err.Error())
	if err := res.WriteErrorString(http.StatusBadRequest, err.Error()); err != nil {
		log.WithError(err).Error("unable to write error")
	}
}
