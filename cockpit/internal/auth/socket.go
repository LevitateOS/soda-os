package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"
)

type socketRequest struct {
	Operation   string `json:"operation"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	NewPassword string `json:"new_password,omitempty"`
}

type socketResponse struct {
	Result string `json:"result"`
}

type Client struct {
	socket string
}

func NewClient(socket string) Client {
	return Client{socket: socket}
}

func (c Client) Authenticate(username, password string) (Result, error) {
	response, err := c.call(socketRequest{Operation: "authenticate", Username: username, Password: password})
	return authenticationResult(response, err)
}

func (c Client) AuthenticatePasswordless(username string) (Result, error) {
	response, err := c.call(socketRequest{Operation: "authenticate_passwordless", Username: username})
	return authenticationResult(response, err)
}

func authenticationResult(response socketResponse, err error) (Result, error) {
	if err != nil {
		return "", err
	}
	result := Result(response.Result)
	if result != Authenticated && result != PasswordChangeRequired {
		return "", errors.New("PAM authentication failed")
	}
	return result, nil
}

func (c Client) ChangePassword(username, currentPassword, newPassword string) error {
	response, err := c.call(socketRequest{Operation: "change_password", Username: username, Password: currentPassword, NewPassword: newPassword})
	if err != nil {
		return err
	}
	if Result(response.Result) != Authenticated {
		return errors.New("PAM password change failed")
	}
	return nil
}

func (c Client) call(request socketRequest) (socketResponse, error) {
	connection, err := net.DialTimeout("unix", c.socket, 5*time.Second)
	if err != nil {
		return socketResponse{}, err
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return socketResponse{}, err
	}
	if err := json.NewEncoder(connection).Encode(request); err != nil {
		return socketResponse{}, err
	}
	var response socketResponse
	if err := json.NewDecoder(connection).Decode(&response); err != nil {
		return socketResponse{}, err
	}
	return response, nil
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
	result := Result("")
	var operationError error
	switch request.Operation {
	case "authenticate":
		result, operationError = authenticator.Authenticate(request.Username, request.Password)
	case "authenticate_passwordless":
		result, operationError = authenticator.AuthenticatePasswordless(request.Username)
	case "change_password":
		operationError = authenticator.ChangePassword(request.Username, request.Password, request.NewPassword)
		if operationError == nil {
			result = Authenticated
		}
	default:
		operationError = errors.New("unsupported authentication operation")
	}
	if operationError != nil {
		log.Printf("authentication operation failed for %q: %v", request.Username, operationError)
	}
	_ = json.NewEncoder(connection).Encode(socketResponse{Result: string(result)})
}
