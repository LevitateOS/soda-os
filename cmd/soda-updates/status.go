package main

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func statusCommand(connect updateFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read native bootc deployment status",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			host, err := connect(command.OutOrStdout(), command.ErrOrStderr()).Status(command.Context())
			if err != nil {
				return err
			}
			return json.NewEncoder(command.OutOrStdout()).Encode(host)
		},
	}
}
