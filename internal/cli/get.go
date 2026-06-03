package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func NewGetCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get /remote ./local",
		Short: "ファイルをダウンロードする",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(args[0], args[1], gf)
		},
	}
}

func runGet(remotePath, localPath string, gf *GlobalFlags) error {
	if strings.HasSuffix(remotePath, "/") {
		return fmt.Errorf("ディレクトリはダウンロードできません。rscli pull を使用してください")
	}

	_, client, err := loadClient(gf)
	if err != nil {
		return err
	}

	body, _, err := client.Get(remotePath)
	if err != nil {
		return err
	}
	defer body.Close()

	// If localPath is a directory, use the remote filename.
	info, statErr := os.Stat(localPath)
	if statErr == nil && info.IsDir() {
		localPath = filepath.Join(localPath, filepath.Base(remotePath))
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("ディレクトリの作成に失敗: %w", err)
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("ファイルの作成に失敗: %w", err)
	}
	defer f.Close()

	n, err := io.Copy(f, body)
	if err != nil {
		return fmt.Errorf("ダウンロード中にエラー: %w", err)
	}

	if gf.Verbose {
		fmt.Fprintf(os.Stderr, "✓ %s → %s (%d bytes)\n", remotePath, localPath, n)
	} else {
		fmt.Printf("✓ %s\n", localPath)
	}
	return nil
}
