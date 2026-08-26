package main

import (
	"log"
	"os"

	"github.com/LevitateOS/soda-os/cockpit/internal/auth"
	"github.com/LevitateOS/soda-os/cockpit/internal/cert"
	"github.com/LevitateOS/soda-os/cockpit/internal/server"
	"github.com/LevitateOS/soda-os/cockpit/internal/soda"
)

func main() {
	address := envOr("SODA_COCKPIT_ADDRESS", ":9090")
	certFile := envOr("SODA_COCKPIT_CERT", "/var/lib/soda/certs/cockpit.crt")
	keyFile := envOr("SODA_COCKPIT_KEY", "/var/lib/soda/certs/cockpit.key")
	socket := envOr("SODA_SOCKET", "/run/soda/sodad.sock")
	pamSocket := envOr("SODA_PAM_SOCKET", "/run/soda/pam.sock")

	if err := cert.Ensure(certFile, keyFile); err != nil {
		log.Fatal(err)
	}
	api, err := soda.NewClient(socket)
	if err != nil {
		log.Fatal(err)
	}
	defer api.Close()
	app, err := server.New(api, auth.NewClient(pamSocket))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("soda-cockpit listening on %s", address)
	if err := app.ListenAndServeTLS(address, certFile, keyFile); err != nil {
		log.Fatal(err)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
