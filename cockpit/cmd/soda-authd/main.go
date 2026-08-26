package main

import (
	"log"
	"os"

	"github.com/LevitateOS/soda-os/cockpit/internal/auth"
)

func main() {
	socket := "/run/soda/pam.sock"
	if value := os.Getenv("SODA_PAM_SOCKET"); value != "" {
		socket = value
	}
	log.Printf("soda-authd listening on %s", socket)
	if err := auth.ListenAndServe(socket, auth.NewPAM()); err != nil {
		log.Fatal(err)
	}
}
