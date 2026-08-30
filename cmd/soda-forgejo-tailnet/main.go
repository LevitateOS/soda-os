// soda-forgejo-tailnet prints the enrolled appliance identity for Forgejo.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/LevitateOS/soda-os/internal/tailnet"
)

func main() {
	endpoint, err := tailnet.New(tailnet.Options{}).Endpoint(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(endpoint.Identity, endpoint.IPv4)
}
