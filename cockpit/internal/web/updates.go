package web

import (
	"net/http"

	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
)

type osUpdatePageView struct {
	pageIdentity
	OSUpdate  *daemonclient.OSUpdateStatus
	OSRelease *daemonclient.OSRelease
	Message   string
	Error     string
}

type osUpdateView struct {
	status  int
	message string
	error   string
	release *daemonclient.OSRelease
	value   *daemonclient.OSUpdateStatus
}

func (s *Server) osUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	s.renderOSUpdate(w, r, osUpdateView{status: http.StatusOK})
}

func (s *Server) checkOSUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	release, err := s.updates.CheckOSUpdate(r.Context())
	if err != nil {
		s.renderOSUpdate(w, r, osUpdateView{status: http.StatusUnprocessableEntity, error: err.Error()})
		return
	}
	message := "This host already runs the current signed Soda OS release."
	if release.Available {
		message = "A signed Soda OS release is available to stage."
	}
	s.renderOSUpdate(w, r, osUpdateView{status: http.StatusOK, message: message, release: &release})
}

func (s *Server) stageOSUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	release, err := s.updates.CheckOSUpdate(r.Context())
	if err != nil {
		s.renderOSUpdate(w, r, osUpdateView{status: http.StatusUnprocessableEntity, error: err.Error()})
		return
	}
	if !release.Available {
		s.renderOSUpdate(w, r, osUpdateView{status: http.StatusUnprocessableEntity, error: "No newer signed Soda OS release is available.", release: &release})
		return
	}
	status, err := s.updates.StageOSUpdate(r.Context(), release.ImageReference)
	if err != nil {
		s.renderOSUpdate(w, r, osUpdateView{status: http.StatusUnprocessableEntity, error: err.Error(), release: &release})
		return
	}
	s.renderOSUpdate(w, r, osUpdateView{status: http.StatusOK, message: "Update downloaded and locked. Running work is unchanged; activation still requires confirmation.", release: &release, value: &status})
}

func (s *Server) activateOSUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid activation confirmation", http.StatusBadRequest)
		return
	}
	if r.FormValue("confirm_reboot") != "yes" {
		s.renderOSUpdate(w, r, osUpdateView{status: http.StatusUnprocessableEntity, error: "Select the maintenance reboot confirmation before activating the update."})
		return
	}
	if err := s.updates.ActivateOSUpdate(r.Context()); err != nil {
		s.renderOSUpdate(w, r, osUpdateView{status: http.StatusUnprocessableEntity, error: err.Error()})
		return
	}
	s.renderOSUpdate(w, r, osUpdateView{status: http.StatusOK, message: "Maintenance reboot requested."})
}

func (s *Server) renderOSUpdate(w http.ResponseWriter, r *http.Request, view osUpdateView) {
	if view.value == nil {
		value, err := s.updates.OSUpdateStatus(r.Context())
		if err != nil {
			s.render(w, http.StatusBadGateway, "os_update.html", osUpdatePageView{pageIdentity: pageIdentity{Title: "OS update · Soda OS", User: currentUser(r)}, Error: "OS update status is unavailable."})
			return
		}
		view.value = &value
	}
	s.render(w, view.status, "os_update.html", osUpdatePageView{pageIdentity: pageIdentity{Title: "OS update · Soda OS", User: currentUser(r)}, OSUpdate: view.value, OSRelease: view.release, Message: view.message, Error: view.error})
}
