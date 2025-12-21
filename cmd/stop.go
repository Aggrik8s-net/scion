package cmd

import (
	"context"
	"fmt"

	"github.com/ptone/gswarm/pkg/runtime"
	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop <agent>",
	Short: "Stop and remove an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]
		rt := runtime.GetRuntime()
		
		fmt.Printf("Stopping agent '%s'...\n", agentName)
		if err := rt.Stop(context.Background(), agentName); err != nil {
			return err
		}

		fmt.Printf("Agent '%s' stopped and removed.\n", agentName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

