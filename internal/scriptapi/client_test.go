package scriptapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/api/option"
	"google.golang.org/api/script/v1"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := New(context.Background(), server.Client(), option.WithEndpoint(server.URL+"/"))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	return client
}

func TestClientCreateProject(t *testing.T) {
	expectedTitle := "Example"
	expectedParent := "parent-123"

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/projects" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req struct {
			Title    string `json:"title"`
			ParentId string `json:"parentId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Title != expectedTitle {
			t.Fatalf("title mismatch: %s", req.Title)
		}
		if req.ParentId != expectedParent {
			t.Fatalf("parentId mismatch: %s", req.ParentId)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"scriptId": "script-123",
			"title":    expectedTitle,
		})
	})

	project, err := client.CreateProject(context.Background(), expectedTitle, expectedParent)
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	if project.ScriptId != "script-123" {
		t.Fatalf("scriptId mismatch: %s", project.ScriptId)
	}
}

func TestClientGetProject(t *testing.T) {
	scriptID := "script-abc"

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/projects/"+scriptID {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"scriptId": scriptID,
			"title":    "Title",
		})
	})

	project, err := client.GetProject(context.Background(), scriptID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if project.ScriptId != scriptID {
		t.Fatalf("scriptId mismatch: %s", project.ScriptId)
	}
}

func TestClientGetContentWithVersion(t *testing.T) {
	scriptID := "script-xyz"
	version := int64(7)

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/projects/"+scriptID+"/content" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("versionNumber"); got != "7" {
			t.Fatalf("versionNumber mismatch: %s", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"scriptId": scriptID,
			"files": []map[string]string{
				{"name": "Code", "type": "SERVER_JS", "source": "function a() {}"},
			},
		})
	})

	content, err := client.GetContent(context.Background(), scriptID, version)
	if err != nil {
		t.Fatalf("GetContent failed: %v", err)
	}
	if content.ScriptId != scriptID {
		t.Fatalf("scriptId mismatch: %s", content.ScriptId)
	}
	if len(content.Files) != 1 || content.Files[0].Name != "Code" {
		t.Fatalf("unexpected files: %v", content.Files)
	}
}

func TestClientUpdateContent(t *testing.T) {
	scriptID := "script-789"
	input := &script.Content{
		Files: []*script.File{
			{
				Name:   "Code",
				Type:   "SERVER_JS",
				Source: "function b() {}",
			},
		},
	}

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/v1/projects/"+scriptID+"/content" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req script.Content
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(req.Files) != 1 || req.Files[0].Name != "Code" {
			t.Fatalf("unexpected request content: %#v", req.Files)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"scriptId": scriptID,
		})
	})

	content, err := client.UpdateContent(context.Background(), scriptID, input)
	if err != nil {
		t.Fatalf("UpdateContent failed: %v", err)
	}
	if content.ScriptId != scriptID {
		t.Fatalf("scriptId mismatch: %s", content.ScriptId)
	}
}

func TestClientCreateVersion(t *testing.T) {
	scriptID := "script-ver"

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/projects/"+scriptID+"/versions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req struct {
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Description != "release note" {
			t.Fatalf("unexpected description: %s", req.Description)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"scriptId":      scriptID,
			"versionNumber": 12,
		})
	})

	version, err := client.CreateVersion(context.Background(), scriptID, "release note")
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}
	if version.ScriptId != scriptID {
		t.Fatalf("scriptId mismatch: %s", version.ScriptId)
	}
	if version.VersionNumber != 12 {
		t.Fatalf("versionNumber mismatch: %d", version.VersionNumber)
	}
}

func TestClientUpdateDeployment(t *testing.T) {
	scriptID := "script-deploy"
	deploymentID := "AKfycb1234"

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/v1/projects/"+scriptID+"/deployments/"+deploymentID {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req struct {
			DeploymentConfig struct {
				VersionNumber int64  `json:"versionNumber"`
				Description   string `json:"description"`
			} `json:"deploymentConfig"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.DeploymentConfig.VersionNumber != 7 {
			t.Fatalf("versionNumber mismatch: %d", req.DeploymentConfig.VersionNumber)
		}
		if req.DeploymentConfig.Description != "hotfix" {
			t.Fatalf("description mismatch: %s", req.DeploymentConfig.Description)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deploymentId": deploymentID,
			"deploymentConfig": map[string]any{
				"versionNumber": 7,
			},
		})
	})

	deployment, err := client.UpdateDeployment(context.Background(), scriptID, deploymentID, &script.DeploymentConfig{
		VersionNumber: 7,
		Description:   "hotfix",
	})
	if err != nil {
		t.Fatalf("UpdateDeployment failed: %v", err)
	}
	if deployment.DeploymentId != deploymentID {
		t.Fatalf("deploymentId mismatch: %s", deployment.DeploymentId)
	}
	if deployment.DeploymentConfig == nil || deployment.DeploymentConfig.VersionNumber != 7 {
		t.Fatalf("deployment config mismatch: %#v", deployment.DeploymentConfig)
	}
}

func TestClientCreateDeployment(t *testing.T) {
	scriptID := "script-create-deploy"

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/projects/"+scriptID+"/deployments" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req struct {
			VersionNumber int64  `json:"versionNumber"`
			Description   string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.VersionNumber != 3 {
			t.Fatalf("versionNumber mismatch: %d", req.VersionNumber)
		}
		if req.Description != "release" {
			t.Fatalf("description mismatch: %s", req.Description)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deploymentId": "dep-new",
			"deploymentConfig": map[string]any{
				"versionNumber": 3,
				"description":   "release",
			},
		})
	})

	deployment, err := client.CreateDeployment(context.Background(), scriptID, &script.DeploymentConfig{
		VersionNumber: 3,
		Description:   "release",
	})
	if err != nil {
		t.Fatalf("CreateDeployment failed: %v", err)
	}
	if deployment.DeploymentId != "dep-new" {
		t.Fatalf("deploymentId mismatch: %s", deployment.DeploymentId)
	}
}

func TestClientListDeployments(t *testing.T) {
	scriptID := "script-list-deploy"

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/projects/"+scriptID+"/deployments" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployments": []map[string]any{
				{
					"deploymentId": "dep-1",
					"deploymentConfig": map[string]any{
						"versionNumber": 8,
						"description":   "prod",
					},
				},
			},
		})
	})

	deployments, err := client.ListDeployments(context.Background(), scriptID)
	if err != nil {
		t.Fatalf("ListDeployments failed: %v", err)
	}
	if len(deployments) != 1 {
		t.Fatalf("expected one deployment, got %d", len(deployments))
	}
	if deployments[0].DeploymentId != "dep-1" {
		t.Fatalf("deploymentId mismatch: %s", deployments[0].DeploymentId)
	}
}

func TestClientListDeploymentsPaginatesAllPages(t *testing.T) {
	scriptID := "script-list-deploy-pages"
	requestCount := 0

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/projects/"+scriptID+"/deployments" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		pageToken := r.URL.Query().Get("pageToken")
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		switch pageToken {
		case "":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"deployments": []map[string]any{
					{"deploymentId": "dep-1"},
				},
				"nextPageToken": "next-token",
			})
		case "next-token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"deployments": []map[string]any{
					{"deploymentId": "dep-2"},
				},
			})
		default:
			t.Fatalf("unexpected pageToken: %q", pageToken)
		}
	})

	deployments, err := client.ListDeployments(context.Background(), scriptID)
	if err != nil {
		t.Fatalf("ListDeployments failed: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}
	if len(deployments) != 2 {
		t.Fatalf("expected two deployments, got %d", len(deployments))
	}
	if deployments[0].DeploymentId != "dep-1" || deployments[1].DeploymentId != "dep-2" {
		t.Fatalf("unexpected deployments: %#v", deployments)
	}
}

func TestClientRunFunction(t *testing.T) {
	scriptID := "script-run"

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/scripts/"+scriptID+":run" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req struct {
			Function   string `json:"function"`
			DevMode    bool   `json:"devMode"`
			Parameters []any  `json:"parameters"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Function != "hello" {
			t.Fatalf("function mismatch: %s", req.Function)
		}
		if !req.DevMode {
			t.Fatalf("expected devMode true")
		}
		if len(req.Parameters) != 2 {
			t.Fatalf("parameters mismatch: %#v", req.Parameters)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"done":     true,
			"response": map[string]any{"result": "ok"},
		})
	})

	op, err := client.RunFunction(context.Background(), scriptID, "hello", []any{"a", 1}, true)
	if err != nil {
		t.Fatalf("RunFunction failed: %v", err)
	}
	if !op.Done {
		t.Fatalf("expected done=true")
	}
}
