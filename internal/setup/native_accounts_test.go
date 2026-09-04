package setup

import (
	"reflect"
	"testing"
)

func TestWheelMembers(t *testing.T) {
	got, err := wheelMembers("wheel:x:10:grace,ada\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"grace", "ada"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("wheelMembers() = %#v, want %#v", got, want)
	}
}

func TestWheelMembersRejectsInvalidRecord(t *testing.T) {
	if _, err := wheelMembers("wheel:x:10:bad name\n"); err == nil {
		t.Fatal("invalid member was accepted")
	}
}
