package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/LevitateOS/soda-os/cockpit/internal/appliance"
	"github.com/LevitateOS/soda-os/cockpit/internal/auth"
	"github.com/LevitateOS/soda-os/cockpit/internal/cert"
	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
	"github.com/LevitateOS/soda-os/cockpit/internal/web"
	"github.com/LevitateOS/soda-os/internal/tailnet"
)

func main() {
	address := envOr("SODA_COCKPIT_ADDRESS", ":9090")
	certFile := envOr("SODA_COCKPIT_CERT", "/var/lib/soda/certs/cockpit.crt")
	keyFile := envOr("SODA_COCKPIT_KEY", "/var/lib/soda/certs/cockpit.key")
	socket := envOr("SODA_SOCKET", "/run/soda/sodad.sock")
	pamSocket := envOr("SODA_PAM_SOCKET", "/run/soda/pam.sock")

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("read installed hostname: %v", err)
	}
	identity, err := appliance.FromHostname(hostname)
	if err != nil {
		log.Fatalf("resolve appliance address: %v", err)
	}
	if magicDNSName, identityErr := tailnet.New(tailnet.Options{}).Identity(context.Background()); identityErr != nil {
		log.Printf("Tailnet identity is unavailable; dashboard is limited to local-console access until enrollment: %v", identityErr)
	} else {
		identity, err = appliance.FromTailnet(hostname, magicDNSName)
		if err != nil {
			log.Fatalf("resolve Tailnet appliance identity: %v", err)
		}
	}
	if err := cert.Ensure(certFile, keyFile, identity); err != nil {
		log.Fatal(err)
	}
	api, err := daemonclient.NewClient(socket)
	if err != nil {
		log.Fatal(err)
	}
	defer api.Close()
	forgejoURL := ""
	if endpoint, endpointErr := tailnet.New(tailnet.Options{}).Endpoint(context.Background()); endpointErr == nil {
		forgejoURL = fmt.Sprintf("http://%s:3000", endpoint.Identity)
	}
	app, err := web.New(web.Ports{Accounts: api, Projects: api, Host: api, Updates: api}, auth.NewClient(pamSocket), identity.Address, forgejoURL)
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
