package bitbucketapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNewClient_RequiresToken(t *testing.T) {
	t.Parallel()

	_, err := NewClient("")
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestListProjects(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		if request.URL.Path != "/rest/api/latest/projects" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}

		query := request.URL.Query()
		if query.Get("start") != "0" || query.Get("limit") != "25" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}

		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("unexpected authorization header: %q", got)
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"size":1,"limit":25,"isLastPage":true,"values":[{"key":"PRJ","id":1,"name":"Project"}],"start":0,"nextPageStart":0}`))
	}))
	defer server.Close()

	client, err := NewClient("test-token", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	page, err := client.ListProjects(context.Background(), 0, 25)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}

	if len(page.Values) != 1 || page.Values[0].Key != "PRJ" {
		t.Fatalf("unexpected response: %+v", page.Values)
	}
}

func TestCreateRepository_DefaultScmIDAndBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		if request.URL.Path != "/rest/api/latest/projects/PRJ/repos" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}

		bodyBytes, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}

		if payload["scmId"] != "git" {
			t.Fatalf("expected default scmId=git, got %v", payload["scmId"])
		}
		if payload["name"] != "my-repo" {
			t.Fatalf("unexpected name: %v", payload["name"])
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"slug":"my-repo","id":123,"name":"my-repo","forkable":true,"public":false,"project":{"key":"PRJ","id":1,"name":"Project"}}`))
	}))
	defer server.Close()

	client, err := NewClient("test-token", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	repo, err := client.CreateRepository(context.Background(), "PRJ", CreateRepositoryRequest{Name: "my-repo", Forkable: true})
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}

	if repo.ID != 123 || repo.Slug != "my-repo" {
		t.Fatalf("unexpected repository response: %+v", repo)
	}
}

func TestListRepositories_ReturnsAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	client, err := NewClient("test-token", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.ListRepositories(context.Background(), "PRJ", 0, 10)
	if err == nil {
		t.Fatal("expected API error")
	}

	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewClient_InvalidBaseURL(t *testing.T) {
	t.Parallel()

	_, err := NewClient("token", WithBaseURL("://bad-url"))
	if err == nil {
		t.Fatal("expected invalid base URL error")
	}

	if _, parseErr := url.Parse("://bad-url"); parseErr == nil {
		t.Fatal("test precondition expected parse error")
	}
}
