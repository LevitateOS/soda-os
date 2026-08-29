package web

import (
	"net/http"

	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
)

type homeView struct {
	pageIdentity
	projectListView
	Host      *daemonclient.HostStatus
	HostError string
	Address   string
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projects, err := s.visibleProjects(r.Context(), user)
	if err != nil {
		http.Error(w, "load projects", http.StatusBadGateway)
		return
	}
	data := homeView{pageIdentity: pageIdentity{Title: "Soda OS", User: user}, Address: s.address}
	data.ProjectCards, err = s.projectCards(r.Context(), projects)
	if err != nil {
		data.ProjectsError = "Projects are temporarily unavailable."
	}
	if user.Role == daemonclient.RoleAdmin {
		host, hostErr := s.host.HostStatus(r.Context())
		if hostErr != nil {
			data.HostError = "Host status is temporarily unavailable."
		} else {
			data.Host = &host
		}
	}
	s.render(w, http.StatusOK, "index.html", data)
}
