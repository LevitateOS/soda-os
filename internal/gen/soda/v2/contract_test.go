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
		"CreateSshDeviceKey",
		"ListSshDeviceKeys",
		"RevokeSshDeviceKey",
		"CreateProject",
		"ListProjects",
		"ListProjectsForPerson",
		"AddCollaborator",
		"ListCollaborators",
		"ListWorktrees",
		"GetDeployKey",
		"GetProjectToolchain",
		"StartProvisioning",
		"ListProvisioningJobs",
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

func TestPersonalWorkspaceIdentityContract(t *testing.T) {
	person := (&Person{}).ProtoReflect().Descriptor()
	require.Nil(t, person.Fields().ByName("ssh_public_key"))
	request := (&CreateProjectRequest{}).ProtoReflect().Descriptor()
	require.True(t, request.Fields().ByName("initial_person_ids").IsList())
}

func TestProjectSourceUsesOneofWithoutKindDiscriminator(t *testing.T) {
	descriptor := (&ProjectSource{}).ProtoReflect().Descriptor()
	require.Nil(t, descriptor.Fields().ByName("kind"))
	require.Equal(t, 1, descriptor.Oneofs().Len())
	require.Equal(t, protoreflect.Name("source"), descriptor.Oneofs().Get(0).Name())
	require.Equal(t, descriptor.Oneofs().Get(0), descriptor.Fields().ByName("empty").ContainingOneof())
	require.Equal(t, descriptor.Oneofs().Get(0), descriptor.Fields().ByName("git").ContainingOneof())
}
