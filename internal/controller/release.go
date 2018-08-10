package controller

import (
	log "github.com/Sirupsen/logrus"
	"github.com/ghodss/yaml"
	"k8s.io/helm/pkg/chartutil"
	hapi_chart "k8s.io/helm/pkg/proto/hapi/chart"
	tiller "k8s.io/helm/pkg/proto/hapi/services"

	"fmt"
	"github.com/AcalephStorage/rudder/internal/client"
)

// GetReleaseResponse contains the response for requesting Release information
type GetReleaseResponse struct {
	Content *tiller.GetReleaseContentResponse `json:"content"`
	Status  *tiller.GetReleaseStatusResponse  `json:"status"`
}

// ReleaseController handles helm release related operations
type ReleaseController struct {
	tillerClient   *client.TillerClient
	repoController *RepoController
}

// NewReleaseController creates a new Release controller
func NewReleaseController(tillerClient *client.TillerClient, repoController *RepoController) *ReleaseController {
	return &ReleaseController{
		tillerClient:   tillerClient,
		repoController: repoController,
	}
}

// ListReleases returns a list of releases
func (rc *ReleaseController) ListReleases(req *tiller.ListReleasesRequest) (*tiller.ListReleasesResponse, error) {
	res, err := rc.tillerClient.ListReleases(req)
	if err != nil {
		log.WithError(err).Error("unable to get list of releases from tiller")
		return nil, err
	}
	return res, nil
}

// InstallRelease installs a new release of the provided chart
func (rc *ReleaseController) InstallRelease(name, namespace, repo, chart, version string, values map[string]interface{}, wait bool, timeout int64) (*tiller.InstallReleaseResponse, error) {
	chartDetails, err := rc.repoController.ChartDetails(repo, chart, version)
	if err != nil {
		log.WithError(err).Error("unable to get chart details")
		return nil, err
	}
	tarball := chartDetails.ChartFile

	inChart, err := chartutil.LoadFile(tarball)
	if err != nil {
		log.WithError(err).Error("unable to load chart details")
		return nil, err
	}
	raw, _ := yaml.Marshal(values)

	inValues := make(map[string]*hapi_chart.Value)
	for k, v := range values {
		inValues[k] = &hapi_chart.Value{Value: fmt.Sprintf("%v", v)}
	}

	config := &hapi_chart.Config{
		Raw:    string(raw),
		Values: inValues,
	}

	req := &tiller.InstallReleaseRequest{
		Name:      name,
		Namespace: namespace,
		Chart:     inChart,
		Values:    config,
		Wait:      wait,
		Timeout:   timeout,
	}

	res, err := rc.tillerClient.InstallRelease(req)
	if err != nil {
		log.WithError(err).Error("unable to install new release")
		return nil, err
	}
	return res, nil
}

// UninstallRelease uninstall a release
func (rc *ReleaseController) UninstallRelease(releaseName string, purge bool) (*tiller.UninstallReleaseResponse, error) {
	req := &tiller.UninstallReleaseRequest{
		Name:  releaseName,
		Purge: purge,
	}

	res, err := rc.tillerClient.UninstallRelease(req)
	if err != nil {
		log.WithError(err).Error("unable to uninstall release")
		return nil, err
	}
	return res, nil
}

// GetRelease returns the release details
func (rc *ReleaseController) GetRelease(name string, version int32) (*GetReleaseResponse, error) {
	req := &tiller.GetReleaseContentRequest{
		Name:    name,
		Version: version,
	}
	content, err := rc.tillerClient.GetReleaseContent(req)
	if err != nil {
		log.WithError(err).Error("unable to get release content")
		return nil, err
	}

	req2 := &tiller.GetReleaseStatusRequest{
		Name:    name,
		Version: version,
	}
	status, err := rc.tillerClient.GetReleaseStatus(req2)
	if err != nil {
		log.WithError(err).Error("unable to get release status")
		return nil, err
	}
	return &GetReleaseResponse{
		Content: content,
		Status:  status,
	}, nil
}

// Updates a already installed release of the provided chart
func (rc *ReleaseController) UpdateRelease(name string, chart string, repo string, version string, values map[string]interface{}, dryRun bool, disableHooks bool, recreate bool, timeout int64, resetValues bool, wait bool, reuseValues bool, force bool) (*tiller.UpdateReleaseResponse, error) {
	chartDetails, err := rc.repoController.ChartDetails(repo, chart, version)
	if err != nil {
		log.WithError(err).Error("unable to get chart details")
		return nil, err
	}
	tarball := chartDetails.ChartFile

	inChart, err := chartutil.LoadFile(tarball)
	if err != nil {
		log.WithError(err).Error("unable to load chart details")
		return nil, err
	}
	raw, _ := yaml.Marshal(values)

	inValues := make(map[string]*hapi_chart.Value)
	for k, v := range values {
		inValues[k] = &hapi_chart.Value{Value: fmt.Sprintf("%v", v)}
	}

	config := &hapi_chart.Config{
		Raw:    string(raw),
		Values: inValues,
	}

	req := &tiller.UpdateReleaseRequest{
		Name:         name,
		Chart:        inChart,
		Values:       config,
		DryRun:       dryRun,
		DisableHooks: disableHooks,
		Recreate:     recreate,
		Timeout:      timeout,
		ResetValues:  resetValues,
		Wait:         wait,
		ReuseValues:  reuseValues,
		Force:        force,
	}

	res, err := rc.tillerClient.UpdateRelease(req)
	if err != nil {
		log.WithError(err).Error("unable to update release")
		return nil, err
	}
	return res, nil
}

// Rollback a already installed release of the provided chart
func (rc *ReleaseController) RollbackRelease(name string, dryRun bool, disableHooks bool, version int32, recreate bool, timeout int64, wait bool, force bool) (*tiller.RollbackReleaseResponse, error) {

	req := &tiller.RollbackReleaseRequest{
		Name:         name,
		DryRun:       dryRun,
		DisableHooks: disableHooks,
		Version:      version,
		Recreate:     recreate,
		Timeout:      timeout,
		Wait:         wait,
		Force:        force,
	}

	res, err := rc.tillerClient.RollbackRelease(req)
	if err != nil {
		log.WithError(err).Error("unable to update release")
		return nil, err
	}
	return res, nil
}
