package auth

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"
)

type socketRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type socketResponse struct {
	Authenticated bool `json:"authenticated"`
}

type Client struct {
	socket string
}

func NewClient(socket string) Client {
	return Client{socket: socket}
}

func (c Client) Authenticate(username, password string) error {
	connection, err := net.DialTimeout("unix", c.socket, 5*time.Second)
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	if err := json.NewEncoder(connection).Encode(socketRequest{Username: username, Password: password}); err != nil {
		return err
	}
	var response socketResponse
	if err := json.NewDecoder(connection).Decode(&response); err != nil {
		return err
	}
	if !response.Authenticated {
		return errors.New("PAM authentication failed")
	}
	return nil
}

func ListenAndServe(socket string, authenticator Authenticator) error {
	if err := os.MkdirAll(filepath.Dir(socket), 0o770); err != nil {
		return err
	}
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	if err := os.Chmod(socket, 0o660); err != nil {
		return err
	}
	return Serve(listener, authenticator)
}

func Serve(listener net.Listener, authenticator Authenticator) error {
	for {
		connection, err := listener.Accept()
		if err != nil {
			return err
		}
		go serveConnection(connection, authenticator)
	}
}

func serveConnection(connection net.Conn, authenticator Authenticator) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	var request socketRequest
	if err := json.NewDecoder(connection).Decode(&request); err != nil {
		return
	}
	authenticated := authenticator.Authenticate(request.Username, request.Password) == nil
	_ = json.NewEncoder(connection).Encode(socketResponse{Authenticated: authenticated})
}
