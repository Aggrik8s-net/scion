package cmd

import (
	"context"
	"fmt"

	"github.com/ptone/gswarm/pkg/runtime"
	"github.com/spf13/cobra"
)

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs <agent>",
	Short: "Get logs of an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]
		rt := runtime.GetRuntime()
		
		logs, err := rt.GetLogs(context.Background(), agentName)
		if err != nil {
			return err
		}

		fmt.Println(logs)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
