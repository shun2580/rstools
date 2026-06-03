package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shun2580/rstools/internal/remotestorage"
)

func NewRmCmd(gf *GlobalFlags) *cobra.Command {
	var recursive bool

	cmd := &cobra.Command{
		Use:   "rm /path",
		Short: "ファイルまたはディレクトリを削除する",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRm(args[0], recursive, gf)
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "ディレクトリを再帰的に削除する")
	return cmd
}

func runRm(path string, recursive bool, gf *GlobalFlags) error {
	_, client, err := loadClient(gf)
	if err != nil {
		return err
	}

	if recursive || strings.HasSuffix(path, "/") {
		return runRmDir(client, path, gf)
	}
	if err := client.Delete(path); err != nil {
		return err
	}
	fmt.Printf("✓ 削除: %s\n", path)
	return nil
}

func runRmDir(client *remotestorage.Client, path string, gf *GlobalFlags) error {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	entries, err := client.ListDir(path)
	if err != nil {
		return err
	}

	if !gf.NoInteractive {
		fmt.Printf("%s 以下のファイルをすべて削除します。続けますか？ [y/N]: ", path)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "y") {
			fmt.Println("中断しました")
			return nil
		}
	}

	var errs []string
	for _, e := range entries {
		fullPath := path + e.Name
		if e.IsDir {
			if err := runRmDir(client, fullPath, gf); err != nil {
				errs = append(errs, err.Error())
			}
		} else {
			if err := client.Delete(fullPath); err != nil {
				errs = append(errs, err.Error())
			} else if gf.Verbose {
				fmt.Printf("✓ 削除: %s\n", fullPath)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("削除中にエラーが発生しました:\n%s", strings.Join(errs, "\n"))
	}
	fmt.Printf("✓ %s を削除しました\n", path)
	return nil
}
