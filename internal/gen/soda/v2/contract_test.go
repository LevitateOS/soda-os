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

func TestProjectSourceUsesOneofWithoutKindDiscriminator(t *testing.T) {
	descriptor := (&ProjectSource{}).ProtoReflect().Descriptor()
	require.Nil(t, descriptor.Fields().ByName("kind"))
	require.Equal(t, 1, descriptor.Oneofs().Len())
	require.Equal(t, protoreflect.Name("source"), descriptor.Oneofs().Get(0).Name())
	require.Equal(t, descriptor.Oneofs().Get(0), descriptor.Fields().ByName("empty").ContainingOneof())
	require.Equal(t, descriptor.Oneofs().Get(0), descriptor.Fields().ByName("git").ContainingOneof())
}

func TestSubscribeEventsResponseHasExplicitRefreshControl(t *testing.T) {
	descriptor := (&SubscribeEventsResponse{}).ProtoReflect().Descriptor()
	require.Equal(t, 1, descriptor.Oneofs().Len())
	require.Equal(t, protoreflect.Name("payload"), descriptor.Oneofs().Get(0).Name())
	require.Equal(t, descriptor.Oneofs().Get(0), descriptor.Fields().ByName("event").ContainingOneof())
	require.Equal(t, descriptor.Oneofs().Get(0), descriptor.Fields().ByName("control").ContainingOneof())
	require.NotEqual(t, StreamControl_STREAM_CONTROL_UNSPECIFIED, StreamControl_STREAM_CONTROL_REFRESH)

	response := &SubscribeEventsResponse{
		Payload: &SubscribeEventsResponse_Control{Control: StreamControl_STREAM_CONTROL_REFRESH},
	}
	require.Equal(t, StreamControl_STREAM_CONTROL_REFRESH, response.GetControl())
	require.Nil(t, response.GetEvent())
}
