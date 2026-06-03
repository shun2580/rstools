package cli

import "github.com/spf13/cobra"

// GlobalFlags holds flag values shared across all commands.
type GlobalFlags struct {
	Verbose       bool
	NoInteractive bool
	Insecure      bool
}

// AddGlobalFlags registers common flags on a command.
func AddGlobalFlags(cmd *cobra.Command, f *GlobalFlags) {
	cmd.PersistentFlags().BoolVarP(&f.Verbose, "verbose", "v", false, "詳細ログを出力する")
	cmd.PersistentFlags().BoolVar(&f.NoInteractive, "no-interactive", false, "対話プロンプトを無効にする")
	cmd.PersistentFlags().BoolVar(&f.Insecure, "insecure", false, "TLS証明書の検証をスキップする（開発用）")
}
