package runners

import "path/filepath"

type LaunchCommand struct {
	Path      string
	Arguments []string
	Directory string
	Home      string
}

func (native *Native) Launch(id string) (LaunchCommand, error) {
	descriptor, err := native.readDescriptor(id)
	if err != nil {
		return LaunchCommand{}, err
	}
	state := native.statePath(id)
	if descriptor.Provider == ProviderForgejo {
		return LaunchCommand{
			Path:      "/usr/bin/forgejo-runner",
			Arguments: []string{"forgejo-runner", "daemon", "--config", filepath.Join(state, "forgejo-runner.yml")},
			Directory: state,
			Home:      state,
		}, nil
	}
	app := filepath.Join(state, "actions-runner")
	return LaunchCommand{
		Path:      filepath.Join(app, "run.sh"),
		Arguments: []string{"run.sh"},
		Directory: app,
		Home:      state,
	}, nil
}
