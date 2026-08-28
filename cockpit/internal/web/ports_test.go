package web

import (
	"context"
	"errors"

	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
)

type fakePorts struct {
	accounts fakeAccounts
	projects fakeProjects
	host     fakeHost
	updates  fakeUpdates
}

type fakeAccounts struct {
	people       []daemonclient.Person
	keys         []daemonclient.SSHDeviceKey
	created      *daemonclient.CreatePersonRequest
	keyPersonIDs []string
}

func (f *fakeAccounts) People(context.Context) ([]daemonclient.Person, error) { return f.people, nil }
func (f *fakeAccounts) CreatePerson(_ context.Context, request daemonclient.CreatePersonRequest) error {
	f.created = &request
	return nil
}
func (f *fakeAccounts) SSHDeviceKeys(_ context.Context, personID string) ([]daemonclient.SSHDeviceKey, error) {
	f.keyPersonIDs = append(f.keyPersonIDs, personID)
	var keys []daemonclient.SSHDeviceKey
	for _, key := range f.keys {
		if key.PersonID == personID {
			keys = append(keys, key)
		}
	}
	return keys, nil
}
func (f *fakeAccounts) CreateSSHDeviceKey(_ context.Context, personID, label, publicKey, hint string) error {
	key := daemonclient.SSHDeviceKey{ID: "key-new", PersonID: personID, Label: label, PublicKey: publicKey, Fingerprint: "SHA256:new", IdentityFileHint: hint}
	f.keys = append(f.keys, key)
	return nil
}
func (f *fakeAccounts) RevokeSSHDeviceKey(_ context.Context, personID, keyID string) error {
	for index, key := range f.keys {
		if key.PersonID == personID && key.ID == keyID {
			f.keys = append(f.keys[:index], f.keys[index+1:]...)
			return nil
		}
	}
	return errors.New("key not found")
}

type fakeProjects struct {
	projects  []daemonclient.Project
	members   []daemonclient.Person
	worktrees []daemonclient.Worktree
	jobs      []daemonclient.ProvisioningJob
	toolchain *daemonclient.ToolchainInstallation
	created   *daemonclient.CreateProjectRequest
	retried   bool
}

func (f *fakeProjects) Projects(context.Context) ([]daemonclient.Project, error) {
	return f.projects, nil
}
func (f *fakeProjects) ProjectsForPerson(context.Context, string) ([]daemonclient.Project, error) {
	return f.projects, nil
}
func (f *fakeProjects) CreateProject(_ context.Context, request daemonclient.CreateProjectRequest) (daemonclient.Project, error) {
	f.created = &request
	return daemonclient.Project{ID: "project-1"}, nil
}
func (f *fakeProjects) Members(context.Context, string) ([]daemonclient.Person, error) {
	return f.members, nil
}
func (f *fakeProjects) AddCollaborator(context.Context, daemonclient.AddCollaboratorCommand) error {
	return nil
}
func (f *fakeProjects) Worktrees(context.Context, string) ([]daemonclient.Worktree, error) {
	return f.worktrees, nil
}
func (f *fakeProjects) Jobs(context.Context, string) ([]daemonclient.ProvisioningJob, error) {
	return f.jobs, nil
}
func (f *fakeProjects) RetryProvisioning(context.Context, string) error {
	f.retried = true
	job := daemonclient.ProvisioningJob{ID: "job-new", ProjectID: "project-1", State: "installing"}
	f.jobs = append([]daemonclient.ProvisioningJob{job}, f.jobs...)
	return nil
}
func (f *fakeProjects) Toolchain(context.Context, string) (*daemonclient.ToolchainInstallation, error) {
	return f.toolchain, nil
}
func (f *fakeProjects) DeployKey(context.Context, string) (string, error) { return "", nil }

type fakeHost struct {
	calls int
}

func (f *fakeHost) HostStatus(context.Context) (daemonclient.HostStatus, error) {
	f.calls++
	return daemonclient.HostStatus{Health: daemonclient.HostHealth{Overall: "ready"}}, nil
}

type fakeUpdates struct {
	status        daemonclient.OSUpdateStatus
	release       daemonclient.OSRelease
	stagedImage   string
	activateCalls int
}

func (f *fakeUpdates) OSUpdateStatus(context.Context) (daemonclient.OSUpdateStatus, error) {
	return f.status, nil
}
func (f *fakeUpdates) CheckOSUpdate(context.Context) (daemonclient.OSRelease, error) {
	return f.release, nil
}
func (f *fakeUpdates) StageOSUpdate(_ context.Context, imageReference string) (daemonclient.OSUpdateStatus, error) {
	f.stagedImage = imageReference
	return f.status, nil
}
func (f *fakeUpdates) ActivateOSUpdate(context.Context) error {
	f.activateCalls++
	return nil
}
