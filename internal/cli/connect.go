package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shun2580/rstools/internal/auth"
)

func NewConnectCmd(gf *GlobalFlags) *cobra.Command {
	var scope string
	var yes bool

	cmd := &cobra.Command{
		Use:   "connect user@host",
		Short: "remoteStorageサーバーに接続・認証する",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnect(args[0], scope, yes, gf)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "*:rw", "OAuthスコープ")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "既存トークンの上書きを確認なしで実行する")
	return cmd
}

func runConnect(userHost, scope string, yes bool, gf *GlobalFlags) error {
	store, err := auth.NewTokenStore()
	if err != nil {
		return fmt.Errorf("トークンストアの初期化に失敗: %w", err)
	}

	// Check for existing token.
	existing, err := store.Load()
	if err != nil {
		return err
	}
	if existing != nil && !yes {
		if gf.NoInteractive {
			return fmt.Errorf("既存のトークンがあります。--yes で上書きするか rscli disconnect を実行してください")
		}
		fmt.Print("既存のトークンがあります。上書きしますか？ [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "y") {
			fmt.Println("中断しました")
			return nil
		}
	}

	// WebFinger discovery.
	fmt.Printf("WebFinger で %s を検索中...\n", userHost)
	ep, err := auth.Discover(userHost, gf.Insecure)
	if err != nil {
		if gf.NoInteractive {
			return fmt.Errorf("WebFinger に失敗しました: %w", err)
		}
		fmt.Printf("WebFinger に失敗しました: %v\n", err)
		ep, err = promptEndpoints()
		if err != nil {
			return err
		}
	}

	// Dynamic client registration (RFC 7591).
	clientID, err := auth.RegisterClient(ep.RegEndpoint, "http://127.0.0.1/callback", gf.Insecure)
	if err != nil {
		if gf.NoInteractive {
			return fmt.Errorf("クライアント登録に失敗しました (--no-interactive): %w", err)
		}
		fmt.Printf("動的クライアント登録に失敗しました: %v\n", err)
		fmt.Print("client_id を手動入力してください: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		clientID = strings.TrimSpace(line)
		if clientID == "" {
			return fmt.Errorf("client_id が空です")
		}
	}

	// OAuth2 Authorization Code + PKCE flow.
	fmt.Println("ブラウザで認証を開始します...")
	token, err := auth.Flow(ep, clientID, scope, gf.Insecure)
	if err != nil {
		return fmt.Errorf("認証に失敗しました: %w", err)
	}

	if err := store.Save(token); err != nil {
		return fmt.Errorf("トークンの保存に失敗しました: %w", err)
	}

	fmt.Printf("✓ 認証完了 (scope: %s)\n", token.Scope)
	fmt.Printf("  ストレージURL: %s\n", token.StorageURL)
	return nil
}

func promptEndpoints() (*auth.Endpoints, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("ストレージURL を入力してください: ")
	storageURL, _ := reader.ReadString('\n')
	storageURL = strings.TrimSpace(storageURL)
	if storageURL == "" {
		return nil, fmt.Errorf("ストレージURL が空です")
	}
	fmt.Print("認可エンドポイント (auth_endpoint) を入力してください: ")
	authEP, _ := reader.ReadString('\n')
	fmt.Print("トークンエンドポイント (token_endpoint) を入力してください: ")
	tokenEP, _ := reader.ReadString('\n')
	return &auth.Endpoints{
		StorageURL:    storageURL,
		AuthEndpoint:  strings.TrimSpace(authEP),
		TokenEndpoint: strings.TrimSpace(tokenEP),
	}, nil
}
