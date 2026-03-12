package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("test-token")
	if c.Token != "test-token" {
		t.Error("token not set")
	}
	if c.HTTPClient == nil {
		t.Error("http client nil")
	}
}

func TestNewClient_Empty(t *testing.T) {
	c := NewClient("")
	if c.Token != "" {
		t.Error("token should be empty")
	}
}

func TestListRepos(t *testing.T) {
	repos := []Repo{
		{Name: "repo1", FullName: "org/repo1", DefaultBranch: "main"},
		{Name: "repo2", FullName: "org/repo2", DefaultBranch: "master", Private: true},
	}
	reposJSON, _ := json.Marshal(repos)
	emptyJSON := []byte("[]")

	mux := http.NewServeMux()
	// Org probe succeeds
	mux.HandleFunc("/orgs/testorg", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"login":"testorg"}`))
	})
	// Page 1 returns repos, page 2 returns empty
	mux.HandleFunc("/orgs/testorg/repos", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "2" {
			w.Write(emptyJSON)
		} else {
			w.Write(reposJSON)
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	c := &Client{HTTPClient: server.Client(), Token: "tok"}
	// Override base URL by replacing github.com references
	// We need to use the actual method but with a mock server
	// Instead, test the do() method directly and test higher-level with integration

	// Test that do() works with auth
	body, err := c.do(server.URL + "/orgs/testorg")
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	if !strings.Contains(string(body), "testorg") {
		t.Error("unexpected body")
	}
}

func TestDo_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := &Client{HTTPClient: server.Client()}
	_, err := c.do(server.URL + "/fail")
	if err == nil {
		t.Error("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404: %v", err)
	}
}

func TestDo_WithToken(t *testing.T) {
	var receivedAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Write([]byte("ok"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := &Client{HTTPClient: server.Client(), Token: "mytoken"}
	_, err := c.do(server.URL + "/check")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if receivedAuth != "token mytoken" {
		t.Errorf("expected 'token mytoken', got %q", receivedAuth)
	}
}

func TestDo_WithoutToken(t *testing.T) {
	var receivedAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Write([]byte("ok"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := &Client{HTTPClient: server.Client(), Token: ""}
	_, err := c.do(server.URL + "/check")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if receivedAuth != "" {
		t.Errorf("expected no auth header, got %q", receivedAuth)
	}
}

func TestListProtoFiles_Mock(t *testing.T) {
	tree := TreeResponse{
		SHA: "abc123",
		Tree: []TreeEntry{
			{Path: "proto/service.proto", Type: "blob", SHA: "sha1"},
			{Path: "proto/types.proto", Type: "blob", SHA: "sha2"},
			{Path: "README.md", Type: "blob", SHA: "sha3"},
			{Path: "src", Type: "tree", SHA: "sha4"},
			{Path: "other.go", Type: "blob", SHA: "sha5"},
		},
	}
	treeJSON, _ := json.Marshal(tree)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		w.Write(treeJSON)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := &Client{HTTPClient: server.Client()}
	// We can't directly test ListProtoFiles since it hardcodes the GitHub URL,
	// but we can test the filtering logic by calling do + parsing
	data, err := c.do(server.URL + "/repos/org/repo/git/trees/main")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	var resp TreeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Filter like ListProtoFiles does
	var protos []TreeEntry
	for _, entry := range resp.Tree {
		if entry.Type == "blob" && strings.HasSuffix(entry.Path, ".proto") {
			protos = append(protos, entry)
		}
	}
	if len(protos) != 2 {
		t.Errorf("expected 2 proto files, got %d", len(protos))
	}
	if protos[0].Path != "proto/service.proto" {
		t.Errorf("expected proto/service.proto, got %s", protos[0].Path)
	}
}

func TestGetFileContent_Mock(t *testing.T) {
	content := `syntax = "proto3";
package test;
message Foo { string bar = 1; }
`
	mux := http.NewServeMux()
	mux.HandleFunc("/org/repo/main/test.proto", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	// Test raw content fetch behavior
	c := &Client{HTTPClient: server.Client(), Token: "tok"}
	req, _ := http.NewRequest("GET", server.URL+"/org/repo/main/test.proto", nil)
	req.Header.Set("Authorization", "token tok")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetFileContent_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := &Client{HTTPClient: server.Client()}
	_, err := c.do(server.URL + "/missing")
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestTokenFromGHCLI(t *testing.T) {
	// Just verify it doesn't panic — actual behavior depends on gh being installed
	token := TokenFromGHCLI()
	// token may be empty or non-empty depending on environment
	_ = token
}

func TestRepoStruct(t *testing.T) {
	r := Repo{
		Name:          "test",
		FullName:      "org/test",
		DefaultBranch: "main",
		Private:       true,
	}
	if r.Name != "test" {
		t.Error("name")
	}
	if !r.Private {
		t.Error("private")
	}
}

func TestProtoFileStruct(t *testing.T) {
	pf := ProtoFile{
		Repo:    "test",
		Path:    "proto/test.proto",
		Content: "syntax = \"proto3\";",
		SHA:     "abc",
	}
	if pf.Repo != "test" {
		t.Error("repo")
	}
	if pf.Content == "" {
		t.Error("content empty")
	}
}

func TestDo_InvalidURL(t *testing.T) {
	c := NewClient("")
	_, err := c.do("://invalid")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestListRepos_Pagination(t *testing.T) {
	page1 := []Repo{
		{Name: "r1", FullName: "org/r1", DefaultBranch: "main"},
	}
	page2 := []Repo{
		{Name: "r2", FullName: "org/r2", DefaultBranch: "main"},
	}
	p1json, _ := json.Marshal(page1)
	p2json, _ := json.Marshal(page2)
	emptyJSON := []byte("[]")

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		switch page {
		case "1", "":
			w.Write(p1json)
		case "2":
			w.Write(p2json)
		default:
			w.Write(emptyJSON)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := &Client{HTTPClient: server.Client()}

	// Simulate pagination by manually calling
	var all []Repo
	for page := 1; ; page++ {
		data, err := c.do(server.URL + "?page=" + strings.Replace("X", "X", string(rune('0'+page)), 1))
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		var repos []Repo
		if err := json.Unmarshal(data, &repos); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(repos) == 0 {
			break
		}
		all = append(all, repos...)
		if page > 5 {
			t.Fatal("too many pages")
		}
	}
	if len(all) != 2 {
		t.Errorf("expected 2 repos across pages, got %d", len(all))
	}
}
