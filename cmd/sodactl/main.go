package main

import (
	"fmt"
	"os"
)

func main() {
	if err := newApp().command().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "sodactl:", err)
		os.Exit(1)
	}
}
