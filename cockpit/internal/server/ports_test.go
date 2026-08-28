package server

import (
	"context"
	"errors"

	"github.com/LevitateOS/soda-os/cockpit/internal/soda"
)

type fakePorts struct {
	accounts fakeAccounts
	projects fakeProjects
	host     fakeHost
	updates  fakeUpdates
}

type fakeAccounts struct {
	people       []soda.Person
	keys         []soda.SSHDeviceKey
	created      *soda.CreatePersonRequest
	keyPersonIDs []string
}

func (f *fakeAccounts) People(context.Context) ([]soda.Person, error) { return f.people, nil }
func (f *fakeAccounts) CreatePerson(_ context.Context, request soda.CreatePersonRequest) error {
	f.created = &request
	return nil
}
func (f *fakeAccounts) SSHDeviceKeys(_ context.Context, personID string) ([]soda.SSHDeviceKey, error) {
	f.keyPersonIDs = append(f.keyPersonIDs, personID)
	var keys []soda.SSHDeviceKey
	for _, key := range f.keys {
		if key.PersonID == personID {
			keys = append(keys, key)
		}
	}
	return keys, nil
}
func (f *fakeAccounts) CreateSSHDeviceKey(_ context.Context, personID, label, publicKey, hint string) error {
	key := soda.SSHDeviceKey{ID: "key-new", PersonID: personID, Label: label, PublicKey: publicKey, Fingerprint: "SHA256:new", IdentityFileHint: hint}
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
	projects  []soda.Project
	members   []soda.Person
	worktrees []soda.Worktree
	jobs      []soda.ProvisioningJob
	toolchain *soda.ToolchainInstallation
	created   *soda.CreateProjectRequest
	retried   bool
}

func (f *fakeProjects) Projects(context.Context) ([]soda.Project, error) { return f.projects, nil }
func (f *fakeProjects) ProjectsForPerson(context.Context, string) ([]soda.Project, error) {
	return f.projects, nil
}
func (f *fakeProjects) CreateProject(_ context.Context, request soda.CreateProjectRequest) (soda.Project, error) {
	f.created = &request
	return soda.Project{ID: "project-1"}, nil
}
func (f *fakeProjects) Members(context.Context, string) ([]soda.Person, error) {
	return f.members, nil
}
func (f *fakeProjects) AddCollaborator(context.Context, soda.AddCollaboratorCommand) error {
	return nil
}
func (f *fakeProjects) Worktrees(context.Context, string) ([]soda.Worktree, error) {
	return f.worktrees, nil
}
func (f *fakeProjects) Jobs(context.Context, string) ([]soda.ProvisioningJob, error) {
	return f.jobs, nil
}
func (f *fakeProjects) RetryProvisioning(context.Context, string) error {
	f.retried = true
	job := soda.ProvisioningJob{ID: "job-new", ProjectID: "project-1", State: "installing"}
	f.jobs = append([]soda.ProvisioningJob{job}, f.jobs...)
	return nil
}
func (f *fakeProjects) Toolchain(context.Context, string) (*soda.ToolchainInstallation, error) {
	return f.toolchain, nil
}
func (f *fakeProjects) DeployKey(context.Context, string) (string, error) { return "", nil }

type fakeHost struct {
	calls int
}

func (f *fakeHost) HostStatus(context.Context) (soda.HostStatus, error) {
	f.calls++
	return soda.HostStatus{Health: soda.HostHealth{Overall: "ready"}}, nil
}

type fakeUpdates struct {
	status        soda.OSUpdateStatus
	release       soda.OSRelease
	stagedImage   string
	activateCalls int
}

func (f *fakeUpdates) OSUpdateStatus(context.Context) (soda.OSUpdateStatus, error) {
	return f.status, nil
}
func (f *fakeUpdates) CheckOSUpdate(context.Context) (soda.OSRelease, error) {
	return f.release, nil
}
func (f *fakeUpdates) StageOSUpdate(_ context.Context, imageReference string) (soda.OSUpdateStatus, error) {
	f.stagedImage = imageReference
	return f.status, nil
}
func (f *fakeUpdates) ActivateOSUpdate(context.Context) error {
	f.activateCalls++
	return nil
}
