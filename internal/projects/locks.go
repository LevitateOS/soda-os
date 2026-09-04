package projects

import (
	"errors"
	"fmt"
	"io"
)

func closeLockWithError(lock io.Closer, operationErr error, description string) error {
	closeErr := lock.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("unlock %s: %w", description, closeErr)
	}
	return errors.Join(operationErr, closeErr)
}
