package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shun2580/rstools/internal/config"
	"github.com/shun2580/rstools/internal/sync"
)

func NewPullCmd(gf *GlobalFlags) *cobra.Command {
	var noDelete bool
	var force bool
	var dryRun bool
	var parallel int
	var excludes []string
	var resetState bool
	var forceUnlock bool

	cmd := &cobra.Command{
		Use:   "pull /remote ./local",
		Short: "リモート → ローカルへ同期する",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPull(args[0], args[1], sync.Options{
				NoDelete: noDelete,
				Force:    force,
				DryRun:   dryRun,
				Parallel: parallel,
				Excludes: excludes,
				Verbose:  gf.Verbose,
			}, resetState, forceUnlock, gf)
		},
	}

	cmd.Flags().BoolVar(&noDelete, "no-delete", false, "ローカルへの削除伝播を無効にする")
	cmd.Flags().BoolVar(&force, "force", false, "コンフリクト時に強制上書きする")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "実行内容を表示するだけで変更しない")
	cmd.Flags().IntVar(&parallel, "parallel", 3, "並列転送数")
	cmd.Flags().StringArrayVar(&excludes, "exclude", nil, "除外パターン（複数指定可）")
	cmd.Flags().BoolVar(&resetState, "reset-state", false, "同期状態をリセットして初回同期扱いにする")
	cmd.Flags().BoolVar(&forceUnlock, "force-unlock", false, "ロックファイルを強制解除する")

	return cmd
}

func runPull(remotePath, localDir string, opts sync.Options, resetState, forceUnlock bool, gf *GlobalFlags) error {
	lockFile, err := config.LockFile()
	if err != nil {
		return err
	}
	stateFile, err := config.SyncStateFile()
	if err != nil {
		return err
	}

	if forceUnlock {
		if err := config.ForceUnlock(lockFile); err != nil {
			return err
		}
		fmt.Println("✓ ロックを解除しました")
	}

	if resetState {
		_ = os.Remove(stateFile)
		fmt.Println("✓ 同期状態をリセットしました")
	}

	if !opts.DryRun {
		if err := config.AcquireLock(lockFile); err != nil {
			return err
		}
		defer config.ReleaseLock(lockFile)
	}

	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("ローカルディレクトリの作成に失敗: %w", err)
	}

	_, client, err := loadClient(gf)
	if err != nil {
		return err
	}

	opts.StateFile = stateFile

	fmt.Printf("pull: %s → %s\n", remotePath, localDir)
	if opts.DryRun {
		fmt.Println("（dry-run モード — 実際の変更はありません）")
	}

	summary, err := sync.Pull(client, remotePath, localDir, opts)
	if err != nil {
		return err
	}

	printSummary("pull", summary)

	if len(summary.Errors) > 0 {
		fmt.Fprintln(os.Stderr, "\n以下のエラーが発生しました:")
		for _, e := range summary.Errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		if summary.Downloaded > 0 || summary.Deleted > 0 {
			return fmt.Errorf("部分的に失敗しました（終了コード1）: %s", strings.Join(summary.Errors, "; "))
		}
		return fmt.Errorf("pull に失敗しました: %s", strings.Join(summary.Errors, "; "))
	}
	return nil
}
