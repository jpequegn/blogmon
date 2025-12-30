package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "blogmon",
	Short: "Monitor developer blogs and build a knowledge base",
	Long: `Blogmon fetches posts from developer blogs, extracts insights,
scores content value, and builds a searchable knowledge base.

Pipeline: fetch → extract → score → link → query`,
}

func init() {
	rootCmd.Version = "0.1.0"
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
