package resource

import (
	"strconv"
	"strings"

	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/emicklei/go-restful"
	"k8s.io/helm/pkg/proto/hapi/release"
	tiller "k8s.io/helm/pkg/proto/hapi/services"

	"github.com/AcalephStorage/rudder/internal/controller"
	"github.com/AcalephStorage/rudder/internal/util"
)

var (
	sortByMap = map[string]tiller.ListSort_SortBy{
		"unknown":       tiller.ListSort_UNKNOWN,
		"name":          tiller.ListSort_NAME,
		"last-released": tiller.ListSort_LAST_RELEASED,
	}
	sortOrderMap = map[string]tiller.ListSort_SortOrder{
		"asc":  tiller.ListSort_ASC,
		"desc": tiller.ListSort_DESC,
	}
	statusCodeMap = map[string]release.Status_Code{
		"unknown":    release.Status_UNKNOWN,
		"deployed":   release.Status_DEPLOYED,
		"deleted":    release.Status_DELETED,
		"superseded": release.Status_SUPERSEDED,
		"failed":     release.Status_FAILED,
	}
)

var (
	errFailToListReleases      = restful.NewError(http.StatusBadRequest, "unable to get list of releases")
	errFailToInstallRelease    = restful.NewError(http.StatusInternalServerError, "unable to install releases")
	errFailtToUninstallRelease = restful.NewError(http.StatusInternalServerError, "unable to uninstall releases")
	errFailToGetRelease        = restful.NewError(http.StatusInternalServerError, "unable to get release content and status")
	errFailToUpdateRelease     = restful.NewError(http.StatusInternalServerError, "unable to update release")
	errFailToRollbackRelease   = restful.NewError(http.StatusInternalServerError, "unable to rollback release")
)

// InstallReleaseRequest is the request body needed for installing a new release
type InstallReleaseRequest struct {
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	Repo      string                 `json:"repo"`
	Chart     string                 `json:"chart"`
	Version   string                 `json:"version"`
	Values    map[string]interface{} `json:"values"`
	Wait      bool                   `json:"wait"`
	Timeout   int64                  `json:"timeout"`
}

// UpdateReleaseRequest is the request body needed for updating a release
type UpdateReleaseRequest struct {
	Name         string                 `json:"name"`
	Chart        string                 `json:"chart"`
	Repo         string                 `json:"repo"`
	Version      string                 `json:"version"`
	Values       map[string]interface{} `json:"values"`
	DryRun       bool                   `json:"dryRun"`
	DisableHooks bool                   `json:"disableHooks"`
	Recreate     bool                   `json:"recreate"`
	Timeout      int64                  `json:"timeout"`
	ResetValues  bool                   `json:"resetValues"`
	Wait         bool                   `json:"wait"`
	ReuseValues  bool                   `json:"reuseValues"`
	Force        bool                   `json:"force"`
}

// RollbackReleaseRequest is the request body needed to rollback a release
type RollbackReleaseRequest struct {
	// The name of the release
	Name string `json:"name"`
	// dry_run, if true, will run through the release logic but no create
	DryRun bool `json:"dryRun"`
	// DisableHooks causes the server to skip running any hooks for the rollback
	DisableHooks bool `json:"disableHooks"`
	// Version is the version of the release to deploy.
	Version int32 `json:"version"`
	// Performs pods restart for resources if applicable
	Recreate bool `json:"recreate"`
	// timeout specifies the max amount of time any kubernetes client command can run.
	Timeout int64 `json:"timeout"`
	// wait, if true, will wait until all Pods, PVCs, and Services are in a ready state
	// before marking the release as successful. It will wait for as long as timeout
	Wait bool `json:"wait"`
	// Force resource update through delete/recreate if needed.
	Force bool `json:"wait"`
}

// ReleaseResource represents helm releases
type ReleaseResource struct {
	controller *controller.ReleaseController
}

// NewReleaseResource creates a new ReleaseResource instance
func NewReleaseResource(controller *controller.ReleaseController) *ReleaseResource {
	return &ReleaseResource{controller: controller}
}

// Register registers this to the provided container
func (rr *ReleaseResource) Register(container *restful.Container) {

	ws := new(restful.WebService)
	ws.Path("/api/v1/releases").
		Doc("Helm releases").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	// GET /api/v1/releases
	ws.Route(ws.GET("").To(rr.listReleases).
		Doc("list releases").
		Operation("listReleases").
		Param(ws.QueryParameter("limit", "max number of releases to return")).
		Param(ws.QueryParameter("offset", "last release name that was seen")).
		Param(ws.QueryParameter("sort-by", "sort by: unknown, name, last-released")).
		Param(ws.QueryParameter("filter", "regex to filter releases")).
		Param(ws.QueryParameter("sort-order", "sort order: asc, desc")).
		Param(ws.QueryParameter("status-code", "comma-separated status codes: unknown, deployed, deleted, superseded, failed")).
		Writes(tiller.ListReleasesResponse{}))

	// POST /api/v1/releases
	ws.Route(ws.POST("").To(rr.installRelease).
		Doc("install release. defaults: namespace=default, version=latest.").
		Operation("installRelease").
		Reads(InstallReleaseRequest{}).
		Writes(tiller.InstallReleaseResponse{}))

	// DELETE /api/v1/releases/{release}
	ws.Route(ws.DELETE("/{release}").To(rr.uninstallRelease).
		Doc("uninstall release").
		Operation("uninstallRelease").
		Param(ws.PathParameter("release", "the release name to be deleted")).
		Param(ws.QueryParameter("purge", "purge the release")))

	// GET /api/v1/releases/{release}/{version}
	ws.Route(ws.GET("/{release}/{version}").To(rr.getRelease).
		Doc("get release").
		Operation("getRelease").
		Param(ws.PathParameter("release", "the release name")).
		Param(ws.PathParameter("version", "the release version")).
		Writes(controller.GetReleaseResponse{}))

	// PUT /api/v1/releases
	ws.Route(ws.PUT("").To(rr.updateRelease).
		Doc("update release. defaults: version=latest.").
		Operation("updateRelease").
		Reads(UpdateReleaseRequest{}).
		Writes(tiller.UpdateReleaseResponse{}))

	container.Add(ws)

	ws1 := new(restful.WebService)
	ws1.Path("/api/v1/releases/rollback").
		Doc("Helm releases").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	// PUT /api/v1/releases/rollback
	ws1.Route(ws1.PUT("").To(rr.rollbackRelease).
		Doc("rollback release. defaults: version=latest.").
		Operation("rollbackRelease").
		Reads(RollbackReleaseRequest{}).
		Writes(tiller.RollbackReleaseResponse{}))

	container.Add(ws1)

}

// listReleases returns a list of installed releases
func (rr *ReleaseResource) listReleases(req *restful.Request, res *restful.Response) {
	log.Info("Getting list of releases...")

	limit, _ := strconv.ParseInt(req.QueryParameter("limit"), 10, 64)
	offset := req.QueryParameter("offset")
	sortBy := sortByMap[req.QueryParameter("sort-by")]
	filter := req.QueryParameter("filter")
	sortOrder := sortOrderMap[req.QueryParameter("sort-order")]
	statusCodesRaw := req.QueryParameter("status-code")
	var statusCodes []release.Status_Code
	if len(statusCodesRaw) > 0 {
		scs := strings.Split(statusCodesRaw, ",")
		if len(scs) > 0 {
			statusCodes = make([]release.Status_Code, len(scs))
			for i, s := range scs {
				sc, ok := statusCodeMap[s]
				if ok {
					statusCodes[i] = sc
				}
			}
		}
	}

	request := &tiller.ListReleasesRequest{
		Limit:       limit,
		Offset:      offset,
		SortBy:      sortBy,
		Filter:      filter,
		SortOrder:   sortOrder,
		StatusCodes: statusCodes,
	}

	response, err := rr.controller.ListReleases(request)
	if err != nil {
		errorResponse(res, errFailToListReleases)
		return
	}
	if err := res.WriteEntity(response); err != nil {
		errorResponse(res, errFailToWriteResponse)
	}
}

// installRelease installs the provided release and version to the given namespace
func (rr *ReleaseResource) installRelease(req *restful.Request, res *restful.Response) {
	in := InstallReleaseRequest{
		Namespace: "default",
		Version:   "latest",
	}
	if err := req.ReadEntity(&in); err != nil {
		errorResponse(res, errFailToReadResponse)
		return
	}
	out, err := rr.controller.InstallRelease(in.Name, in.Namespace, in.Repo, in.Chart, in.Version, in.Values, in.Wait, in.Timeout)
	if err != nil {
		errorResponse(res, errFailToInstallRelease)
		return
	}
	if err := res.WriteEntity(out); err != nil {
		errorResponse(res, errFailToWriteResponse)
	}
}

// PUT api/v1/releases {request body passed}
func (rr *ReleaseResource) updateRelease(req *restful.Request, res *restful.Response) {
	in := UpdateReleaseRequest{
		Version: "latest",
	}

	if err := req.ReadEntity(&in); err != nil {
		errorResponse(res, errFailToReadResponse)
		return
	}

	out, err := rr.controller.UpdateRelease(in.Name, in.Chart, in.Repo, in.Version, in.Values, in.DryRun, in.DisableHooks, in.Recreate, in.Timeout, in.ResetValues, in.Wait, in.ReuseValues, in.Force)

	if err != nil {
		errorResponse(res, errFailToUpdateRelease)
		return
	}

	if err := res.WriteEntity(out); err != nil {
		errorResponse(res, errFailToWriteResponse)
	}

}

// uninstallRelease removes the release from the list of releases
func (rr *ReleaseResource) uninstallRelease(req *restful.Request, res *restful.Response) {
	releaseName := req.PathParameter("release")
	_, purge := req.Request.URL.Query()["purge"]
	out, err := rr.controller.UninstallRelease(releaseName, purge)
	if err != nil {
		errorResponse(res, errFailtToUninstallRelease)
		return
	}
	if err := res.WriteEntity(out); err != nil {
		errorResponse(res, errFailToWriteResponse)
	}
}

// getRelease returns the details of the provided release
func (rr *ReleaseResource) getRelease(req *restful.Request, res *restful.Response) {
	name := req.PathParameter("release")
	versionRaw := req.PathParameter("version")
	version := util.ToInt32(versionRaw)

	out, err := rr.controller.GetRelease(name, version)
	if err != nil {
		errorResponse(res, errFailToGetRelease)
		return
	}

	if err := res.WriteEntity(out); err != nil {
		errorResponse(res, errFailToWriteResponse)
	}

}

// Post api/v1/releases/rollback {request body passed}
func (rr *ReleaseResource) rollbackRelease(req *restful.Request, res *restful.Response) {
	in := RollbackReleaseRequest{}

	if err := req.ReadEntity(&in); err != nil {
		errorResponse(res, errFailToReadResponse)
		return
	}

	out, err := rr.controller.RollbackRelease(in.Name, in.DryRun, in.DisableHooks, in.Version, in.Recreate, in.Timeout, in.Wait, in.Force)

	if err != nil {
		errorResponse(res, errFailToRollbackRelease)
		return
	}

	if err := res.WriteEntity(out); err != nil {
		errorResponse(res, errFailToWriteResponse)
	}

}

// GET api/v1/releases/:name/:version/:status {create request body}
func (rr *ReleaseResource) releaseStatus(req *restful.Request, res *restful.Response) {
	// TODO
}

// GET /api/v1/releases/:name/:version {create request body}
func (rr *ReleaseResource) releaseContent(req *restful.Request, res *restful.Response) {
	// TODO
}

// ???? I DUNNOT KNOW
func (rr *ReleaseResource) releaseHistory(req *restful.Request, res *restful.Response) {
	// TODO
}

// GET api/v1/version (do we need this?)
func (rr *ReleaseResource) getVersion(req *restful.Request, res *restful.Response) {
	// TODO
}
