package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/shun2580/rstools/internal/remotestorage"
)

func NewLsCmd(gf *GlobalFlags) *cobra.Command {
	var jsonOut bool
	var recursive bool

	cmd := &cobra.Command{
		Use:   "ls /path",
		Short: "ファイル一覧を表示する",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLs(args[0], jsonOut, recursive, gf)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "JSON形式で出力する")
	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "サブディレクトリを再帰的に表示する")
	return cmd
}

func runLs(path string, jsonOut, recursive bool, gf *GlobalFlags) error {
	_, client, err := loadClient(gf)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	entries, err := client.ListDir(path)
	if err != nil {
		return err
	}

	if recursive {
		entries, err = listRecursive(client, path, entries)
		if err != nil {
			return err
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	if jsonOut {
		return json.NewEncoder(os.Stdout).Encode(entries)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, e := range entries {
		if e.IsDir {
			fmt.Fprintf(w, "%-40s\t%s\n", e.Name, "-")
		} else {
			fmt.Fprintf(w, "%-40s\t%d bytes\t%s\n", e.Name, e.Size, e.LastModified.Format("2006-01-02 15:04:05"))
		}
	}
	return w.Flush()
}

func listRecursive(client *remotestorage.Client, base string, entries []remotestorage.Entry) ([]remotestorage.Entry, error) {
	var result []remotestorage.Entry
	for _, e := range entries {
		if e.IsDir {
			sub, err := client.ListDir(base + e.Name)
			if err != nil {
				return nil, err
			}
			sub, err = listRecursive(client, base+e.Name, sub)
			if err != nil {
				return nil, err
			}
			for i := range sub {
				sub[i].Name = e.Name + sub[i].Name
			}
			result = append(result, sub...)
		} else {
			result = append(result, e)
		}
	}
	return result, nil
}
