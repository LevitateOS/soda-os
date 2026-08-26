package sodav2

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestSodaServiceContract(t *testing.T) {
	service := File_soda_v2_soda_proto.Services().ByName("SodaService")
	require.NotNil(t, service)

	wantMethods := []protoreflect.Name{
		"Health",
		"CreatePerson",
		"ImportPerson",
		"ListPeople",
		"CreateProject",
		"ListProjects",
		"ListProjectsForPerson",
		"AddCollaborator",
		"ListCollaborators",
		"CreateWorktree",
		"ListWorktrees",
		"GetDeployKey",
		"GetProjectToolchain",
		"StartProvisioning",
		"ListProvisioningJobs",
		"GetHostStatus",
		"ListWorktreeStatuses",
		"ListActiveSshConnections",
		"SubscribeEvents",
	}
	require.Equal(t, len(wantMethods), service.Methods().Len())
	for _, methodName := range wantMethods {
		require.NotNilf(t, service.Methods().ByName(methodName), "missing RPC %s", methodName)
	}

	subscribe := service.Methods().ByName("SubscribeEvents")
	require.True(t, subscribe.IsStreamingServer())
	require.False(t, subscribe.IsStreamingClient())
	require.True(t, (&SubscribeEventsRequest{}).ProtoReflect().Descriptor().Fields().ByName("project_id").HasOptionalKeyword())
}
