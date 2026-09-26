package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	gh "github.com/google/go-github/v74/github"
)

func TestCatalogMapsAndPaginatesGitHubRepositories(t *testing.T) {
	// Arrange.
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests++
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("page") == "2" {
			_, _ = response.Write([]byte(`[{"id":2,"name":"beta","full_name":"octo/beta","html_url":"https://github.com/octo/beta","owner":{"login":"octo"},"private":true,"default_branch":"main"}]`))
			return
		}
		response.Header().Set("Link", `<`+serverURL(request)+`?page=2>; rel="next"`)
		_, _ = response.Write([]byte(`[{"id":1,"name":"alpha","full_name":"octo/alpha","html_url":"https://github.com/octo/alpha","description":"First","owner":{"login":"octo"},"default_branch":"main"}]`))
	}))
	defer server.Close()
	client := gh.NewClient(nil)
	baseURL, err := client.BaseURL.Parse(server.URL + "/")
	if err != nil {
		t.Fatalf("parse test URL: %v", err)
	}
	client.BaseURL = baseURL
	catalog := NewCatalog(client)

	// Act.
	repositories, err := catalog.ListForAccount(context.Background())

	// Assert.
	if err != nil {
		t.Fatalf("ListForAccount() error = %v", err)
	}
	if requests != 2 || len(repositories) != 2 {
		t.Fatalf("requests = %d, repositories = %d", requests, len(repositories))
	}
	if repositories[0].Owner != "octo" || repositories[0].Description != "First" || repositories[1].Private != true {
		t.Fatalf("mapped repositories = %#v", repositories)
	}
}

func serverURL(request *http.Request) string {
	return "http://" + request.Host + request.URL.Path
}
