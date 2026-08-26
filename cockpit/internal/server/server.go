package server

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed templates/*.html static/*
var content embed.FS

type Server struct {
	templates *template.Template
	assets    http.Handler
}

type pageData struct {
	Title   string
	Version string
}

func New() (*Server, error) {
	templates, err := template.ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, err
	}
	assetsFS, err := fs.Sub(content, "static")
	if err != nil {
		return nil, err
	}
	return &Server{
		templates: templates,
		assets:    http.FileServer(http.FS(assetsFS)),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", s.assets))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		if err := s.templates.ExecuteTemplate(w, "index.html", pageData{
			Title:   "Soda OS",
			Version: "0.1.0",
		}); err != nil {
			http.Error(w, "render page", http.StatusInternalServerError)
		}
	})
	return mux
}

func (s *Server) ListenAndServeTLS(address, certFile, keyFile string) error {
	return http.ListenAndServeTLS(address, certFile, keyFile, s.Handler())
}
