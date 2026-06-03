package cli

import (
	"fmt"

	"github.com/shun2580/rstools/internal/auth"
	"github.com/shun2580/rstools/internal/remotestorage"
)

// loadClient loads the stored token and returns an authenticated remoteStorage client.
func loadClient(gf *GlobalFlags) (*auth.Token, *remotestorage.Client, error) {
	store, err := auth.NewTokenStore()
	if err != nil {
		return nil, nil, fmt.Errorf("トークンストアの初期化に失敗: %w", err)
	}

	token, err := store.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("トークンの読み込みに失敗: %w", err)
	}
	if token == nil {
		return nil, nil, fmt.Errorf("認証情報がありません。先に rscli connect を実行してください")
	}

	// Refresh if expired.
	if token.IsExpired() && token.RefreshToken != "" {
		if gf.Verbose {
			fmt.Println("アクセストークンが期限切れです。リフレッシュ中...")
		}
		refreshed, err := auth.Refresh(token, gf.Insecure)
		if err != nil {
			if gf.NoInteractive {
				return nil, nil, fmt.Errorf("トークンのリフレッシュに失敗 (終了コード2): %w", err)
			}
			fmt.Printf("トークンのリフレッシュに失敗しました: %v\nrscli connect で再認証してください\n", err)
			return nil, nil, fmt.Errorf("再認証が必要です")
		}
		if err := store.Save(refreshed); err != nil {
			return nil, nil, fmt.Errorf("更新トークンの保存に失敗: %w", err)
		}
		token = refreshed
	}

	client := remotestorage.NewClient(token.StorageURL, token.AccessToken, gf.Insecure)
	return token, client, nil
}
