package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"
	"time"

	"github.com/shun2580/rstools/internal/remotestorage"
	"github.com/shun2580/rstools/internal/transfer"
)

// Options configures push/pull behavior.
type Options struct {
	NoDelete   bool
	Force      bool
	DryRun     bool
	Parallel   int
	Excludes   []string
	Verbose    bool
	StateFile  string
}

// Summary holds the result of a push or pull operation.
type Summary struct {
	Uploaded  int
	Downloaded int
	Deleted   int
	Skipped   int
	Conflicts int
	Errors    []string
}

func (s *Summary) hasErrors() bool { return len(s.Errors) > 0 }

// Push uploads changed local files to remote and propagates deletions.
func Push(client *remotestorage.Client, localDir, remotePath string, opts Options) (*Summary, error) {
	if !strings.HasSuffix(remotePath, "/") {
		remotePath += "/"
	}

	state, err := LoadState(opts.StateFile)
	if err != nil {
		return nil, err
	}

	ignore, err := LoadIgnore(localDir, opts.Excludes)
	if err != nil {
		return nil, err
	}

	// Collect local files.
	localFiles := map[string]os.FileInfo{} // relative path → info
	err = filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(localDir, path)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			fmt.Fprintf(os.Stderr, "warning: スキップ（特殊ファイル）: %s\n", path)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if ignore.Match(rel) {
			return nil
		}
		localFiles[rel] = info
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ローカルディレクトリの走査に失敗: %w", err)
	}

	// Collect remote files for conflict detection.
	remoteFiles := map[string]remotestorage.Entry{} // relative path → entry
	entries, err := client.WalkDir(remotePath)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, fmt.Errorf("リモートディレクトリの取得に失敗: %w", err)
	}
	for _, e := range entries {
		remoteFiles[e.Name] = e
	}

	summary := &Summary{}

	type uploadTask struct {
		rel        string
		localPath  string
		remoteFull string
		info       os.FileInfo
	}
	var tasks []uploadTask

	for rel, info := range localFiles {
		remoteFull := remotePath + rel
		stored, hasState := state.Files[remoteFull]
		remote, hasRemote := remoteFiles[rel]

		localChanged := !hasState || info.ModTime().Unix() != stored.Mtime
		remoteChanged := hasState && hasRemote && remote.ETag != stored.ETag

		switch {
		case localChanged && remoteChanged && !opts.Force:
			// Conflict: both sides changed.
			fmt.Fprintf(os.Stderr, "conflict: %s（スキップ。--force で強制上書き）\n", rel)
			summary.Conflicts++
			summary.Skipped++
			continue

		case !localChanged && !remoteChanged && hasState:
			// No changes on either side.
			summary.Skipped++
			continue

		default:
			tasks = append(tasks, uploadTask{rel, filepath.Join(localDir, filepath.FromSlash(rel)), remoteFull, info})
		}
	}

	// Execute uploads in parallel.
	prog := transfer.NewProgress(len(tasks))
	defer prog.Done()

	type result struct {
		rel       string
		remoteFull string
		mtime     int64
		err       error
	}

	parallel := opts.Parallel
	if parallel <= 0 {
		parallel = 3
	}

	sem := make(chan struct{}, parallel)
	var wg gosync.WaitGroup
	results := make(chan result, len(tasks))

	for _, t := range tasks {
		if opts.DryRun {
			fmt.Printf("[dry-run] upload: %s → %s\n", t.rel, t.remoteFull)
			summary.Uploaded++
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(t uploadTask) {
			defer wg.Done()
			defer func() { <-sem }()

			f, err := os.Open(t.localPath)
			if err != nil {
				results <- result{err: fmt.Errorf("open %s: %w", t.rel, err)}
				return
			}
			defer f.Close()

			ct := transfer.DetectContentType(t.localPath)
			err = client.Put(t.remoteFull, ct, f)
			results <- result{
				rel:        t.rel,
				remoteFull: t.remoteFull,
				mtime:      t.info.ModTime().Unix(),
				err:        err,
			}
			prog.Inc(t.rel)
		}(t)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			summary.Errors = append(summary.Errors, r.err.Error())
			continue
		}
		summary.Uploaded++
		state.Files[r.remoteFull] = FileState{
			Mtime: r.mtime,
			Size:  localFiles[r.rel].Size(),
		}
	}

	// Deletion propagation: local deletions → remote deletions.
	if !opts.NoDelete {
		for remoteFull := range state.Files {
			if !strings.HasPrefix(remoteFull, remotePath) {
				continue
			}
			rel := strings.TrimPrefix(remoteFull, remotePath)
			if _, exists := localFiles[rel]; !exists {
				if opts.DryRun {
					fmt.Printf("[dry-run] delete remote: %s\n", remoteFull)
					summary.Deleted++
					continue
				}
				if err := client.Delete(remoteFull); err != nil {
					summary.Errors = append(summary.Errors, err.Error())
				} else {
					delete(state.Files, remoteFull)
					summary.Deleted++
					if opts.Verbose {
						fmt.Fprintf(os.Stderr, "deleted remote: %s\n", remoteFull)
					}
				}
			}
		}
	}

	if !opts.DryRun {
		if err := state.Save(opts.StateFile); err != nil {
			return summary, fmt.Errorf("同期状態の保存に失敗: %w", err)
		}
	}

	return summary, nil
}

// Pull downloads changed remote files to local and propagates deletions.
func Pull(client *remotestorage.Client, remotePath, localDir string, opts Options) (*Summary, error) {
	if !strings.HasSuffix(remotePath, "/") {
		remotePath += "/"
	}

	state, err := LoadState(opts.StateFile)
	if err != nil {
		return nil, err
	}

	ignore, err := LoadIgnore(localDir, opts.Excludes)
	if err != nil {
		return nil, err
	}

	// Collect remote files.
	remoteFiles := map[string]remotestorage.Entry{}
	entries, err := client.WalkDir(remotePath)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, fmt.Errorf("リモートディレクトリの取得に失敗: %w", err)
	}
	for _, e := range entries {
		remoteFiles[e.Name] = e
	}

	summary := &Summary{}

	type downloadTask struct {
		rel        string
		remoteFull string
		localPath  string
		entry      remotestorage.Entry
	}
	var tasks []downloadTask

	for rel, entry := range remoteFiles {
		if ignore.Match(rel) {
			continue
		}
		remoteFull := remotePath + rel
		localPath := filepath.Join(localDir, filepath.FromSlash(rel))
		stored, hasState := state.Files[remoteFull]

		remoteChanged := !hasState || entry.ETag != stored.ETag

		// Check local change.
		localChanged := false
		if hasState {
			if info, err := os.Stat(localPath); err == nil {
				localChanged = info.ModTime().Unix() != stored.Mtime
			}
		}

		switch {
		case remoteChanged && localChanged && !opts.Force:
			fmt.Fprintf(os.Stderr, "conflict: %s（スキップ。--force で強制上書き）\n", rel)
			summary.Conflicts++
			summary.Skipped++
			continue

		case !remoteChanged && hasState:
			summary.Skipped++
			continue

		default:
			tasks = append(tasks, downloadTask{rel, remoteFull, localPath, entry})
		}
	}

	prog := transfer.NewProgress(len(tasks))
	defer prog.Done()

	parallel := opts.Parallel
	if parallel <= 0 {
		parallel = 3
	}

	type result struct {
		rel        string
		remoteFull string
		entry      remotestorage.Entry
		localPath  string
		err        error
	}

	sem := make(chan struct{}, parallel)
	var wg gosync.WaitGroup
	results := make(chan result, len(tasks))

	for _, t := range tasks {
		if opts.DryRun {
			fmt.Printf("[dry-run] download: %s → %s\n", t.remoteFull, t.localPath)
			summary.Downloaded++
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(t downloadTask) {
			defer wg.Done()
			defer func() { <-sem }()

			body, _, err := client.Get(t.remoteFull)
			if err != nil {
				results <- result{err: fmt.Errorf("get %s: %w", t.rel, err)}
				return
			}
			defer body.Close()

			if err := os.MkdirAll(filepath.Dir(t.localPath), 0755); err != nil {
				results <- result{err: fmt.Errorf("mkdir %s: %w", filepath.Dir(t.localPath), err)}
				return
			}

			f, err := os.Create(t.localPath)
			if err != nil {
				results <- result{err: fmt.Errorf("create %s: %w", t.localPath, err)}
				return
			}
			defer f.Close()

			if _, err := io.Copy(f, body); err != nil {
				results <- result{err: fmt.Errorf("write %s: %w", t.localPath, err)}
				return
			}
			// Set mtime to match remote Last-Modified if available.
			if !t.entry.LastModified.IsZero() {
				_ = os.Chtimes(t.localPath, time.Now(), t.entry.LastModified)
			}

			results <- result{
				rel:        t.rel,
				remoteFull: t.remoteFull,
				entry:      t.entry,
				localPath:  t.localPath,
			}
			prog.Inc(t.rel)
		}(t)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			summary.Errors = append(summary.Errors, r.err.Error())
			continue
		}
		summary.Downloaded++
		info, _ := os.Stat(r.localPath)
		fs := FileState{ETag: r.entry.ETag}
		if info != nil {
			fs.Mtime = info.ModTime().Unix()
			fs.Size = info.Size()
		}
		state.Files[r.remoteFull] = fs
	}

	// Deletion propagation: remote deletions → local deletions.
	if !opts.NoDelete {
		for remoteFull := range state.Files {
			if !strings.HasPrefix(remoteFull, remotePath) {
				continue
			}
			rel := strings.TrimPrefix(remoteFull, remotePath)
			if _, exists := remoteFiles[rel]; !exists {
				localPath := filepath.Join(localDir, filepath.FromSlash(rel))
				if opts.DryRun {
					fmt.Printf("[dry-run] delete local: %s\n", localPath)
					summary.Deleted++
					continue
				}
				if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
					summary.Errors = append(summary.Errors, err.Error())
				} else {
					delete(state.Files, remoteFull)
					summary.Deleted++
					if opts.Verbose {
						fmt.Fprintf(os.Stderr, "deleted local: %s\n", localPath)
					}
				}
			}
		}
	}

	if !opts.DryRun {
		if err := state.Save(opts.StateFile); err != nil {
			return summary, fmt.Errorf("同期状態の保存に失敗: %w", err)
		}
	}

	return summary, nil
}
