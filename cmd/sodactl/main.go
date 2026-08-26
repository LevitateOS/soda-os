package main

import (
	"fmt"
	"os"

	"github.com/LevitateOS/soda-os/internal/sodactl"
)

func main() {
	if err := sodactl.New().Command().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "sodactl:", err)
		os.Exit(1)
	}
}
