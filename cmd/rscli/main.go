package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/shun2580/rstools/internal/cli"
)

func main() {
	gf := &cli.GlobalFlags{}

	root := &cobra.Command{
		Use:   "rscli",
		Short: "remoteStorage CLI ツール",
		Long: `rscli は remoteStorage プロトコル（draft-dejong-remotestorage-22）を操作する
Go 製 CLI ツールです。WebFinger によるサーバー自動検出と OAuth2 + PKCE 認証を
サポートします。`,
		SilenceUsage: true,
	}

	cli.AddGlobalFlags(root, gf)

	root.AddCommand(
		cli.NewConnectCmd(gf),
		cli.NewDisconnectCmd(gf),
		cli.NewLsCmd(gf),
		cli.NewGetCmd(gf),
		cli.NewPutCmd(gf),
		cli.NewRmCmd(gf),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitOtherError)
	}
}
