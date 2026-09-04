package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

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
	if status.Dismissed {
		_, err = fmt.Fprintln(session.output, "Soda Setup is complete. Reopen it from Cockpit when needed.")
		return true, err
	}
	printStatus(session.output, status)
	choice, err := readLine(session.reader, session.output, consoleMenu(status))
	if err != nil || choice == "q" || choice == "Q" {
		return true, err
	}
	if err = session.executeAction(choice, status.Connections); err != nil {
		fmt.Fprintln(session.output, "\nCould not complete that action:", err)
	}
	return false, nil
}

func (session consoleSession) executeAction(choice string, connections []setup.Connection) error {
	switch choice {
	case "1":
		return session.createAdministrator()
	case "2":
		return session.allowLocalNetwork(connections)
	case "3":
		return session.connectTailscale()
	case "4":
		_, err := session.service.Dismiss(session.ctx)
		return err
	default:
		return errors.New("choose one of the displayed actions")
	}
}

func printStatus(output io.Writer, status setup.Status) {
	fmt.Fprintln(output, "\nRequired facts:")
	if len(status.Administrators) == 0 {
		fmt.Fprintln(output, "  Linux administrator: missing")
	}
	for _, administrator := range status.Administrators {
		fmt.Fprintf(output, "  Linux administrator %s: password=%s, SSH key=%s, Forgejo=%s\n",
			administrator.Username, answer[administrator.PasswordSet], answer[administrator.SSHPublicKey], answer[administrator.ForgejoReady])
	}
	fmt.Fprintf(output, "  Tailscale connected: %s\n", answer[status.TailscaleConnected])
	fmt.Fprintf(output, "  Access from the local network: %s\n", answer[status.LocalNetworkAllowed])
	fmt.Fprintf(output, "  Ready to dismiss: %s\n", answer[status.CanDismiss])
}

func consoleMenu(status setup.Status) string {
	menu := "\n1. Create the primary Linux administrator\n" +
		"2. Allow access from the local network.\n" +
		"3. Connect Tailscale\n" +
		"4. Dismiss Soda Setup\n" +
		"q. Leave setup running for the next startup\nChoose: "
	if len(status.Administrators) != 0 {
		menu = strings.Replace(menu, "1. Create the primary Linux administrator", "1. Show administrator status", 1)
	}
	return menu
}

func (session consoleSession) createAdministrator() error {
	status, err := session.service.Status(session.ctx)
	if err != nil || len(status.Administrators) != 0 {
		return err
	}
	username, err := readLine(session.reader, session.output, "Linux username: ")
	if err != nil {
		return err
	}
	password, err := readSecret(session.input, session.output, "Password: ")
	if err != nil {
		return err
	}
	confirmation, err := readSecret(session.input, session.output, "Confirm password: ")
	if err != nil {
		return err
	}
	if password != confirmation {
		return errors.New("passwords do not match")
	}
	key, err := readLine(session.reader, session.output, "SSH public key: ")
	if err != nil {
		return err
	}
	_, err = session.service.CreateAdministrator(session.ctx, setup.AdministratorRequest{Username: username, Password: password, AuthorizedKey: key})
	password, confirmation = "", ""
	return err
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
	authKey, err := readSecret(session.input, session.output, "One-use Tailscale auth key: ")
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
