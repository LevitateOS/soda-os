package sodav2

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestSodaServiceContractContainsOnlyTemporaryRuntimeOperations(t *testing.T) {
	service := File_soda_v2_soda_proto.Services().ByName("SodaService")
	require.NotNil(t, service)

	wantMethods := []protoreflect.Name{
		"Health",
		"GetHostStatus",
		"GetOSUpdateStatus",
		"CheckOSUpdate",
		"StageOSUpdate",
		"ActivateOSUpdate",
	}
	require.Equal(t, len(wantMethods), service.Methods().Len())
	for _, methodName := range wantMethods {
		require.NotNilf(t, service.Methods().ByName(methodName), "missing RPC %s", methodName)
	}
}
