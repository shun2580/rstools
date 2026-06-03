package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/shun2580/rstools/internal/auth"
)

func NewDisconnectCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "disconnect",
		Short: "ローカルのトークンを削除してログアウトする",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDisconnect()
		},
	}
}

func runDisconnect() error {
	store, err := auth.NewTokenStore()
	if err != nil {
		return fmt.Errorf("トークンストアの初期化に失敗: %w", err)
	}
	if err := store.Delete(); err != nil {
		return fmt.Errorf("トークンの削除に失敗: %w", err)
	}
	fmt.Println("✓ ログアウトしました")
	return nil
}
