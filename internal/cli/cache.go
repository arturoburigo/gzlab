package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	localcache "github.com/arturoburigo/gzlab/internal/cache"
)

func newCacheCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the local response cache",
	}
	cmd.AddCommand(newCacheClearCommand())
	return cmd
}

func newCacheClearCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear cached GitLab responses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := localcache.DefaultPath()
			if err != nil {
				return err
			}
			if err := localcache.Clear(path); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Cleared cache at %s\n", path)
			return err
		},
	}
}
