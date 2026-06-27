package remotescan

import (
	"fmt"
	"os"
	"path/filepath"
)

// PullResult records the outcome of pulling one discovered file.
type PullResult struct {
	Remote    DiscoveredFile
	LocalPath string
	Err       error // non-nil if this specific file failed to pull
}

// Pull fetches each discovered file via `cat` and writes it under localRoot,
// mirroring RelPath so multiple remote projects with same-named manifests
// (e.g. two package.json files under different directories) don't collide
// locally. A per-file failure (permission denied, file vanished mid-run)
// does not abort the whole pull — it is recorded on that result and the
// remaining files are still attempted, since a project with 9 of 10
// manifests readable is still worth scanning.
func (c *Client) Pull(files []DiscoveredFile, localRoot string) []PullResult {
	results := make([]PullResult, 0, len(files))
	for _, f := range files {
		localPath := filepath.Join(localRoot, filepath.FromSlash(f.RelPath))
		if err := os.MkdirAll(filepath.Dir(localPath), 0o700); err != nil {
			results = append(results, PullResult{Remote: f, Err: fmt.Errorf("create local dir: %w", err)})
			continue
		}

		data, err := c.runCapturingStderr("cat -- " + shellQuote(f.RemotePath))
		if err != nil {
			results = append(results, PullResult{Remote: f, Err: fmt.Errorf("pull %s: %w", f.RemotePath, err)})
			continue
		}

		if err := os.WriteFile(localPath, []byte(data), 0o600); err != nil {
			results = append(results, PullResult{Remote: f, Err: fmt.Errorf("write local file: %w", err)})
			continue
		}

		results = append(results, PullResult{Remote: f, LocalPath: localPath})
	}
	return results
}
