package main

import (
	"io"
	"testing"

	"github.com/LevitateOS/soda-os/internal/build/installer"
	"github.com/stretchr/testify/require"
)

func TestArtifactFlagOverridesReachTheirOwner(t *testing.T) {
	for _, test := range imageCases()[3:5] {
		t.Run(test.action, func(t *testing.T) {
			args := append([]string{test.action, "--architecture", "aarch64", "--output-dir", "chosen-output"}, test.flags...)
			switch options := test.options.(type) {
			case installer.Options:
				args = append(args, "--tool-lock", "chosen.lock")
				options.OutputDir, options.ToolLock = "chosen-output", "chosen.lock"
				test.options = options
			case installer.QCOW2Options:
				args = append(args, "--tool-lock", "chosen.lock")
				options.OutputDir, options.ToolLock = "chosen-output", "chosen.lock"
				test.options = options
			}
			fake := &recordingImage{}
			command := newCommand(func(spec, _ string, _, _ io.Writer) (imageOperations, error) {
				require.Equal(t, "distro/soda.toml", spec)
				return fake, nil
			})
			command.SetOut(io.Discard)
			command.SetArgs(args)
			require.NoError(t, command.Execute())
			require.Equal(t, test.options, fake.options)
		})
	}
}
