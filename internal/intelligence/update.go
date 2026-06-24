package intelligence

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DefaultCommunitySignaturesURL is the published community signature bundle
// consumed by both `fgctl intel update` and the dashboard's refresh action.
const DefaultCommunitySignaturesURL = "https://raw.githubusercontent.com/mah3sec/forgeguardian-signatures/main/dist/signatures.json"

// UpdateResult reports what a community signature update actually did.
type UpdateResult struct {
	Before int `json:"before"`
	Added  int `json:"added"`
	Total  int `json:"total"`
}

// UpdateFromCommunity downloads the community signature bundle from url,
// merges it into the store at storePath, and saves it. Shared by `fgctl intel
// update` and the dashboard's intelligence-refresh endpoint so both go
// through the same real fetch-and-merge path — no fabricated "queued" state.
func UpdateFromCommunity(storePath, url string) (*UpdateResult, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("signatures bundle not found at %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var incoming SignatureStore
	if err := json.Unmarshal(body, &incoming); err != nil {
		return nil, fmt.Errorf("parse signatures bundle: %w", err)
	}

	existing, err := LoadStore(storePath)
	if err != nil {
		return nil, err
	}

	before := len(existing.Signatures)
	added := MergeSignatures(existing, incoming.Signatures)

	if err := SaveStore(storePath, existing); err != nil {
		return nil, err
	}

	return &UpdateResult{Before: before, Added: added, Total: len(existing.Signatures)}, nil
}
