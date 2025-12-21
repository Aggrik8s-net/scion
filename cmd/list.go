package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/ptone/gswarm/pkg/runtime"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List running gswarm agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		rt := runtime.GetRuntime()
		agents, err := rt.List(context.Background(), map[string]string{
			"gswarm.agent": "true",
		})
		if err != nil {
			return err
		}

		if len(agents) == 0 {
			fmt.Println("No active agents found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tID\tIMAGE")
		for _, a := range agents {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.Name, a.Status, a.ID, a.Image)
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

