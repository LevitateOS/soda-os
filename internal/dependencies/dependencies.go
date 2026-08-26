//go:build dependencies

// Package dependencies records dependencies allocated to later Soda OS Go
// components so the foundation owns and reviews their versions in one place.
package dependencies

import (
	_ "github.com/spf13/cobra"
	_ "golang.org/x/sys/unix"
	_ "gorm.io/driver/sqlite"
	_ "gorm.io/gorm"
)
