package setup

import (
	"reflect"
	"testing"
)

func TestParseForgejoUsers(t *testing.T) {
	users, err := parseForgejoUsers("ID Username Email IsActive IsAdmin\n1 ada ada@localhost true true\n2 grace grace@localhost true false\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []forgejoUser{{Username: "ada", Active: true, Admin: true}, {Username: "grace", Active: true}}
	if !reflect.DeepEqual(users, want) {
		t.Fatalf("parseForgejoUsers() = %#v, want %#v", users, want)
	}
}

func TestForgejoBootstrapConfigurationEnablesRegistrationOnLoopback(t *testing.T) {
	configuration := []byte("[server]\nHTTP_ADDR = 0.0.0.0\n[service]\nDISABLE_REGISTRATION = true\nREQUIRE_SIGNIN_VIEW = true\n")
	got, err := forgejoBootstrapConfiguration(configuration)
	if err != nil {
		t.Fatal(err)
	}
	want := "[server]\nHTTP_ADDR = 127.0.0.1\n[service]\nDISABLE_REGISTRATION = false\nREQUIRE_SIGNIN_VIEW = true\n"
	if string(got) != want {
		t.Fatalf("configuration = %q, want %q", got, want)
	}
}

func TestForgejoBootstrapConfigurationRejectsAmbiguousSetting(t *testing.T) {
	if _, err := forgejoBootstrapConfiguration([]byte("[service]\n")); err == nil {
		t.Fatal("missing registration setting was accepted")
	}
}
