package strictjson

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type testRequest struct {
	ID string `json:"id"`
}

func TestDecodeAcceptsOneKnownObject(t *testing.T) {
	var request testRequest
	require.NoError(t, Decode(strings.NewReader(`{"id":"one"}`), &request))
	require.Equal(t, testRequest{ID: "one"}, request)
}

func TestDecodeRejectsInvalidRequestShapes(t *testing.T) {
	for name, input := range map[string]string{
		"duplicate field": `{"id":"one","id":"two"}`,
		"unknown field":   `{"id":"one","project_id":"site"}`,
		"array":           `[]`,
		"trailing object": `{"id":"one"}{"id":"two"}`,
		"unclosed object": `{"id":"one"`,
	} {
		t.Run(name, func(t *testing.T) {
			var request testRequest
			require.Error(t, Decode(strings.NewReader(input), &request))
		})
	}
}

func TestDecodeRejectsInvalidUTF8AndOversizedRequests(t *testing.T) {
	invalidUTF8 := append([]byte(`{"id":"`), 0xff)
	invalidUTF8 = append(invalidUTF8, []byte(`"}`)...)
	var request testRequest
	require.ErrorContains(t, Decode(bytes.NewReader(invalidUTF8), &request), "valid UTF-8")

	oversized := strings.NewReader(`{"id":"` + strings.Repeat("a", maximumRequestBytes) + `"}`)
	require.ErrorContains(t, Decode(oversized, &request), "exceeds 1 MiB")
}
