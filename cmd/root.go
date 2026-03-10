package cmd

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root cobra command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "qus",
		Short: "Query Understanding Service",
		Long:  "QUS transforms raw user queries into structured search intent.",
	}

	root.AddCommand(newHTTPCmd())

	return root
}
