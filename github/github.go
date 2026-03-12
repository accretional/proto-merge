// Package github provides utilities for scanning GitHub organizations
// and repositories for .proto files using the GitHub API.
package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

type Client struct {
	HTTPClient *http.Client
	Token      string
}

// TokenFromGHCLI attempts to get a token from the gh CLI.
func TokenFromGHCLI() string {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func NewClient(token string) *Client {
	return &Client{
		HTTPClient: http.DefaultClient,
		Token:      token,
	}
}

type Repo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

type TreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

type TreeResponse struct {
	SHA       string      `json:"sha"`
	Tree      []TreeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

type ProtoFile struct {
	Repo     string
	Path     string
	Content  string
	SHA      string
}

func (c *Client) do(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "token "+c.Token)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api %s: %d %s", url, resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// ListRepos returns all repos for a given org/user.
// When authenticated, this includes private repos the token has access to.
func (c *Client) ListRepos(org string) ([]Repo, error) {
	// Determine whether this is an org or user account by probing.
	isOrg := true
	probeURL := fmt.Sprintf("https://api.github.com/orgs/%s", org)
	if _, err := c.do(probeURL); err != nil {
		isOrg = false
	}

	var all []Repo
	page := 1
	for {
		var url string
		if isOrg {
			url = fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=100&type=all&page=%d", org, page)
		} else {
			url = fmt.Sprintf("https://api.github.com/users/%s/repos?per_page=100&type=all&page=%d", org, page)
		}
		data, err := c.do(url)
		if err != nil {
			return nil, fmt.Errorf("listing repos: %w", err)
		}
		var repos []Repo
		if err := json.Unmarshal(data, &repos); err != nil {
			return nil, fmt.Errorf("parsing repos: %w", err)
		}
		if len(repos) == 0 {
			break
		}
		all = append(all, repos...)
		page++
	}
	return all, nil
}

// ListProtoFiles returns all .proto file paths in a repo using the git tree API.
func (c *Client) ListProtoFiles(repo Repo) ([]TreeEntry, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/git/trees/%s?recursive=1", repo.FullName, repo.DefaultBranch)
	data, err := c.do(url)
	if err != nil {
		return nil, fmt.Errorf("listing tree for %s: %w", repo.FullName, err)
	}
	var tree TreeResponse
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, fmt.Errorf("parsing tree for %s: %w", repo.FullName, err)
	}
	var protos []TreeEntry
	for _, entry := range tree.Tree {
		if entry.Type == "blob" && strings.HasSuffix(entry.Path, ".proto") {
			protos = append(protos, entry)
		}
	}
	return protos, nil
}

// GetFileContent fetches the raw content of a file from a repo.
func (c *Client) GetFileContent(repoFullName, path, branch string) (string, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repoFullName, branch, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "token "+c.Token)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("fetching %s/%s: status %d", repoFullName, path, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ScanOrg finds all .proto files across all repos in an org/user.
func (c *Client) ScanOrg(org string) ([]ProtoFile, error) {
	repos, err := c.ListRepos(org)
	if err != nil {
		return nil, err
	}
	var pubCount, privCount int
	for _, r := range repos {
		if r.Private {
			privCount++
		} else {
			pubCount++
		}
	}
	fmt.Printf("  found %d repos (%d public, %d private)\n", len(repos), pubCount, privCount)

	var all []ProtoFile
	for _, repo := range repos {
		if repo.DefaultBranch == "" {
			continue
		}
		entries, err := c.ListProtoFiles(repo)
		if err != nil {
			fmt.Printf("  warning: skipping %s: %v\n", repo.FullName, err)
			continue
		}
		for _, entry := range entries {
			content, err := c.GetFileContent(repo.FullName, entry.Path, repo.DefaultBranch)
			if err != nil {
				fmt.Printf("  warning: could not fetch %s/%s: %v\n", repo.FullName, entry.Path, err)
				continue
			}
			all = append(all, ProtoFile{
				Repo:    repo.Name,
				Path:    entry.Path,
				Content: content,
				SHA:     entry.SHA,
			})
		}
	}
	return all, nil
}
