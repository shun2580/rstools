package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shun2580/rstools/internal/transfer"
)

func NewPutCmd(gf *GlobalFlags) *cobra.Command {
	var contentType string

	cmd := &cobra.Command{
		Use:   "put ./local /remote",
		Short: "ファイルをアップロードする",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPut(args[0], args[1], contentType, gf)
		},
	}
	cmd.Flags().StringVar(&contentType, "content-type", "", "Content-Typeを明示指定する")
	return cmd
}

func runPut(localPath, remotePath string, contentType string, gf *GlobalFlags) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("ファイルが見つかりません: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("ディレクトリはアップロードできません。rscli push を使用してください")
	}

	if strings.HasSuffix(remotePath, "/") {
		return fmt.Errorf("リモートパスはファイルパスを指定してください（/ で終わらせないでください）")
	}

	_, client, err := loadClient(gf)
	if err != nil {
		return err
	}

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("ファイルを開けません: %w", err)
	}
	defer f.Close()

	ct := contentType
	if ct == "" {
		ct = transfer.DetectContentType(localPath)
	}

	if err := client.Put(remotePath, ct, f); err != nil {
		return err
	}

	if gf.Verbose {
		fmt.Fprintf(os.Stderr, "✓ %s → %s (%s)\n", localPath, remotePath, ct)
	} else {
		fmt.Printf("✓ %s\n", remotePath)
	}
	return nil
}
