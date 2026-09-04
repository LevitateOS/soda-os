//go:build linux

package runners

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

func (ExecCommandRunner) RunSecret(ctx context.Context, request Command, secret string) error {
	master, slave, err := openNoEchoPTY()
	if err != nil {
		return err
	}
	defer master.Close()
	defer slave.Close()
	command := exec.CommandContext(ctx, request.Name, request.Args...)
	command.Dir = request.Directory
	command.Env = request.Environment
	command.Stdin, command.Stdout, command.Stderr = slave, slave, slave
	command.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, Setctty: true, Ctty: 0,
		Credential: &syscall.Credential{Uid: request.UID, Gid: request.GID},
	}
	if err = command.Start(); err != nil {
		return err
	}
	_ = slave.Close()
	drained := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, master)
		close(drained)
	}()
	if _, err = io.WriteString(master, secret+"\n"); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return err
	}
	err = command.Wait()
	_ = master.Close()
	<-drained
	return err
}

func openNoEchoPTY() (*os.File, *os.File, error) {
	masterFD, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open registration terminal: %w", err)
	}
	master := os.NewFile(uintptr(masterFD), "/dev/ptmx")
	if err = unix.IoctlSetPointerInt(masterFD, unix.TIOCSPTLCK, 0); err != nil {
		master.Close()
		return nil, nil, fmt.Errorf("unlock registration terminal: %w", err)
	}
	number, err := unix.IoctlGetInt(masterFD, unix.TIOCGPTN)
	if err != nil {
		master.Close()
		return nil, nil, fmt.Errorf("identify registration terminal: %w", err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, nil, fmt.Errorf("open registration terminal peer: %w", err)
	}
	termios, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err != nil {
		master.Close()
		slave.Close()
		return nil, nil, fmt.Errorf("read registration terminal settings: %w", err)
	}
	termios.Lflag &^= unix.ECHO | unix.ECHONL
	if err = unix.IoctlSetTermios(int(slave.Fd()), unix.TCSETS, termios); err != nil {
		master.Close()
		slave.Close()
		return nil, nil, fmt.Errorf("disable registration terminal echo: %w", err)
	}
	return master, slave, nil
}
