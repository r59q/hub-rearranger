package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	gh "github.com/google/go-github/v74/github"

	"github.com/r59q/hub-rearranger/services/issues/internal/domain"
)

func TestCatalogMapsIssuesAndRelationships(t *testing.T) {
	// Arrange.
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/repos/octo/demo/issues":
			if request.URL.Query().Get("sort") != "updated" || request.URL.Query().Get("direction") != "desc" {
				t.Errorf("query = %s", request.URL.RawQuery)
			}
			_, _ = response.Write([]byte(`[
				{"id":1,"number":12,"title":"Newest issue","body":"Details","state":"open","html_url":"https://github.com/octo/demo/issues/12","updated_at":"2026-09-27T10:00:00Z","labels":[{"name":"bug","color":"d73a4a"}]},
				{"id":2,"number":13,"title":"A pull request","pull_request":{"html_url":"https://github.com/octo/demo/pull/13"}}
			]`))
		case "/repos/octo/demo/issues/12/sub_issues":
			_, _ = response.Write([]byte(`[{
				"id":3,"number":14,"title":"Child","state":"closed","html_url":"https://github.com/octo/demo/issues/14","repository_url":"https://api.github.com/repos/octo/demo"
			}]`))
		case "/repos/octo/demo/issues/12/timeline":
			_, _ = response.Write([]byte(`[
				{"event":"cross-referenced","source":{"issue":{"id":4,"number":15,"title":"Related","state":"open","html_url":"https://github.com/octo/other/issues/15","repository_url":"https://api.github.com/repos/octo/other"}}},
				{"event":"cross-referenced","source":{"issue":{"id":5,"number":16,"title":"Fix","state":"open","html_url":"https://github.com/octo/demo/pull/16","repository_url":"https://api.github.com/repos/octo/demo","pull_request":{"html_url":"https://github.com/octo/demo/pull/16"}}}}
			]`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	client := gh.NewClient(nil)
	baseURL, err := client.BaseURL.Parse(server.URL + "/")
	if err != nil {
		t.Fatalf("parse test URL: %v", err)
	}
	client.BaseURL = baseURL
	catalog := NewCatalog(client)
	repository := domain.Repository{Owner: "octo", Name: "demo"}

	// Act.
	issues, err := catalog.ListByRepository(context.Background(), repository, 6)
	if err != nil {
		t.Fatalf("ListByRepository() error = %v", err)
	}
	details, err := catalog.GetDetails(context.Background(), repository, 12)

	// Assert.
	if err != nil {
		t.Fatalf("GetDetails() error = %v", err)
	}
	if len(issues) != 1 || issues[0].Title != "Newest issue" || len(issues[0].Labels) != 1 {
		t.Fatalf("issues = %#v", issues)
	}
	if len(details.Subtasks) != 1 || details.Subtasks[0].State != "closed" {
		t.Fatalf("subtasks = %#v", details.Subtasks)
	}
	if len(details.LinkedIssues) != 1 || details.LinkedIssues[0].Repository != "octo/other" || len(details.LinkedPullRequests) != 1 {
		t.Fatalf("linked issues = %#v, pull requests = %#v", details.LinkedIssues, details.LinkedPullRequests)
	}
}
