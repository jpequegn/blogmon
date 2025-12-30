package cmd

import (
	"fmt"
	"os"

	"github.com/julienpequegnot/blogmon/internal/config"
	"github.com/julienpequegnot/blogmon/internal/database"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize blogmon configuration and database",
	Long:  `Creates the ~/.blogmon directory with config.yaml and SQLite database.`,
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	dir := config.Dir()

	// Create directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create config
	cfg := config.Default()
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	fmt.Printf("Created config at %s/config.yaml\n", dir)

	// Create database
	db, err := database.New(config.DBPath())
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	db.Close()
	fmt.Printf("Created database at %s/blogmon.db\n", dir)

	fmt.Println("\nBlogmon initialized! Next steps:")
	fmt.Println("  blogmon add <blog-url>    Add a blog to monitor")
	fmt.Println("  blogmon fetch             Fetch posts from all blogs")

	return nil
}
