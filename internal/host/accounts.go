package host

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/LevitateOS/soda-os/internal/domain"
)

func (s *System) CreatePerson(ctx context.Context, person domain.Person, password string) (Cleanup, error) {
	if strings.ContainsAny(password, "\r\n\x00") {
		return nil, errors.New("password contains a line or NUL delimiter")
	}
	if utf8.RuneCountInString(password) < 6 {
		return nil, errors.New("password must contain at least 6 characters")
	}
	if _, err := s.Runner.Run(ctx, "useradd", []string{"--create-home", "--shell", "/sbin/nologin", person.Username}, nil, ""); err != nil {
		return nil, err
	}
	cleanup := func(cleanupContext context.Context) error {
		_, err := s.Runner.Run(cleanupContext, "userdel", []string{"--remove", person.Username}, nil, "")
		return err
	}
	if _, err := s.Runner.Run(ctx, "chpasswd", nil, nil, person.Username+":"+password+"\n"); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	if _, err := s.Runner.Run(ctx, "chage", []string{"--lastday", "0", person.Username}, nil, ""); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	return cleanup, nil
}

func (s *System) ImportPerson(ctx context.Context, person domain.Person) (Cleanup, error) {
	if _, err := s.Runner.Run(ctx, "getent", []string{"passwd", person.Username}, nil, ""); err != nil {
		return nil, fmt.Errorf("%w: Linux account %s", ErrNotFound, person.Username)
	}
	return func(context.Context) error { return nil }, nil
}
