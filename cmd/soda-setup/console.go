package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/LevitateOS/soda-os/internal/setup"
	"golang.org/x/term"
)

type lineReader interface {
	ReadString(byte) (string, error)
}

var answer = map[bool]string{false: "no", true: "yes"}

func runConsole(ctx context.Context, service setup.Service, input io.Reader, output io.Writer) error {
	session := consoleSession{ctx: ctx, service: service, input: input, output: output, reader: bufio.NewReader(input)}
	return session.run()
}

type consoleSession struct {
	ctx     context.Context
	service setup.Service
	input   io.Reader
	output  io.Writer
	reader  lineReader
}

func (session consoleSession) run() error {
	if _, err := fmt.Fprintln(session.output, "\nSoda Setup\n=========="); err != nil {
		return err
	}
	for {
		done, err := session.step()
		if err != nil || done {
			return err
		}
	}
}

func (session consoleSession) step() (bool, error) {
	status, err := session.service.Status(session.ctx)
	if err != nil {
		return false, err
	}
	printStatus(session.output, status)
	choice, err := readLine(session.reader, session.output, consoleMenu(status))
	if errors.Is(err, io.EOF) || choice == "q" || choice == "Q" {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	if err = session.executeAction(choice, status.Connections); err != nil {
		return true, err
	}
	return true, nil
}

func (session consoleSession) executeAction(choice string, connections []setup.Connection) error {
	switch choice {
	case "1":
		return session.allowLocalNetwork(connections)
	case "2":
		return session.connectTailscale()
	default:
		return errors.New("choose one of the displayed actions")
	}
}

func printStatus(output io.Writer, status setup.Status) {
	fmt.Fprintln(output, "\nNative network access:")
	if len(status.Administrators) == 0 {
		fmt.Fprintln(output, "  Linux administrator: missing")
	}
	for _, administrator := range status.Administrators {
		fmt.Fprintf(output, "  Existing Linux administrator: %s\n", administrator.Username)
	}
	fmt.Fprintf(output, "  Tailscale connected: %s\n", answer[status.TailscaleConnected])
	fmt.Fprintf(output, "  Access from the local network: %s\n", answer[status.LocalNetworkAllowed])
	fmt.Fprintf(output, "  Network configured: %s\n", answer[status.Ready])
}

func consoleMenu(_ setup.Status) string {
	return "\n1. Allow access from the local network\n2. Connect Tailscale\nq. Return to the shell\nChoose: "
}

func (session consoleSession) allowLocalNetwork(connections []setup.Connection) error {
	if len(connections) == 0 {
		return errors.New("NetworkManager reports no active connections")
	}
	fmt.Fprintln(session.output, "\nActive NetworkManager connections:")
	for index, connection := range connections {
		fmt.Fprintf(session.output, "  %d. %s (allowed: %s)\n", index+1, connection.Name, answer[connection.LocalNetworkAllowed])
	}
	selected, err := readLine(session.reader, session.output, "Allow access from the local network on connection: ")
	if err != nil {
		return err
	}
	index, err := strconv.Atoi(selected)
	if err != nil || index < 1 || index > len(connections) {
		return errors.New("choose an active connection number")
	}
	_, err = session.service.AllowLocalNetwork(session.ctx, connections[index-1].Name)
	return err
}

func (session consoleSession) connectTailscale() error {
	authKey, err := readSecret(session.input, session.output, "Tailscale auth key: ")
	if err != nil {
		return err
	}
	_, err = session.service.ConnectTailscale(session.ctx, authKey)
	authKey = ""
	return err
}

func readSecret(input io.Reader, output io.Writer, prompt string) (string, error) {
	if _, err := fmt.Fprint(output, prompt); err != nil {
		return "", err
	}
	file, ok := input.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return "", errors.New("secret input requires an interactive terminal")
	}
	secret, err := term.ReadPassword(int(file.Fd()))
	fmt.Fprintln(output)
	if err != nil {
		return "", err
	}
	return string(secret), nil
}
