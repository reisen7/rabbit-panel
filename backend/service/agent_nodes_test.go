package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"

	"rabbit-panel/model"
)

func TestParseToolCommandKeepsNodeSeparate(t *testing.T) {
	name, args, node, ok := parseToolCommand(`run_container("redis", "myredis", "-p 6379:6379", node="edge")`)
	if !ok || name != "run_container" || node != "edge" {
		t.Fatalf("parsed %v %q %q ok=%v", args, name, node, ok)
	}
	if len(args) != 3 || args[0] != "redis" || args[1] != "myredis" || args[2] != "-p 6379:6379" {
		t.Fatalf("args %v", args)
	}
	name, args, node, ok = parseToolCommand("stop_container(id=abc, 节点=edge)")
	if !ok || name != "stop_container" || node != "edge" || len(args) != 1 || args[0] != "abc" {
		t.Fatalf("stop parsed %v %q %q ok=%v", args, name, node, ok)
	}
	if _, _, _, ok = parseToolCommand("list_containers"); ok {
		t.Fatal("missing parentheses should fail")
	}
}

func TestExecuteToolOnNamedNode(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		switch r.URL.Path {
		case "/api/containers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"abc123456789","name":"redis","status":"Up 1 minute"}]`))
		case "/api/containers/action":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dockerStub := &listOnlyDocker{}
	nodes := NewNodeService(dockerStub, nil, "master", "secret", "local:3958", "127.0.0.1", "3958")
	nodes.RegisterNode(&model.NodeInfo{
		ID:      "edge:3958",
		Name:    "edge",
		Address: srv.Listener.Addr().String(),
		Mode:    model.ModeWorker,
		Status:  model.NodeStatusOnline,
	})
	svc := &AgentService{dockerRepo: dockerStub, nodes: nodes, jwtSecret: []byte("test-secret")}

	got := svc.executeTool(context.Background(), "list_containers(node=edge)")
	if !strings.Contains(got, "redis") || !strings.Contains(got, "节点 edge") || strings.Contains(got, "本机") {
		t.Fatalf("list reply %q path %s", got, gotPath)
	}
	if gotPath != "/api/containers" || !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Fatalf("path %s auth %s", gotPath, gotAuth)
	}

	ctx := context.WithValue(context.Background(), agentNodeKey{}, "edge:3958")
	got = svc.executeTool(ctx, "stop_container(redis)")
	if !strings.Contains(got, "stopped") || !strings.Contains(gotBody, `"action":"stop"`) || !strings.Contains(gotBody, "redis") {
		t.Fatalf("stop reply %q body %s", got, gotBody)
	}

	missing := svc.executeTool(context.Background(), "list_containers(node=missing)")
	if !strings.Contains(missing, "节点不存在") || !strings.Contains(missing, "edge") {
		t.Fatalf("missing %q", missing)
	}
}

func TestNodeMentionAndLocalMaster(t *testing.T) {
	dockerStub := &listOnlyDocker{containers: []types.Container{{
		ID:     "bf82a246af3c0000",
		Names:  []string{"/rabbit-panel"},
		Status: "Up 3 minutes",
	}}}
	nodes := NewNodeService(dockerStub, nil, "master", "secret", "local:3958", "127.0.0.1", "3958")
	nodes.RegisterNode(&model.NodeInfo{
		ID: "edge:3958", Name: "edge", Address: "192.0.2.8:3958", Mode: model.ModeWorker, Status: model.NodeStatusOnline,
	})
	svc := &AgentService{dockerRepo: dockerStub, nodes: nodes, jwtSecret: []byte("test-secret")}
	if got := svc.withMentionedNode(fallbackToolCommand("查看 edge 上有哪些容器"), "查看 edge 上有哪些容器"); got != "list_containers(node=edge)" {
		t.Fatalf("mention %q", got)
	}
	if got := svc.withMentionedNode(fallbackToolCommand("查看有哪些容器"), "查看有哪些容器"); got != "list_containers()" {
		t.Fatalf("plain %q", got)
	}

	before := dockerStub.calls
	got := svc.executeTool(context.Background(), "list_containers(node=local:3958)")
	if !strings.Contains(got, "rabbit-panel") || !strings.Contains(got, "本机") {
		t.Fatalf("local %q", got)
	}
	if dockerStub.calls <= before {
		t.Fatalf("expected local docker list, calls %d -> %d", before, dockerStub.calls)
	}

	prompt := svc.nodePrompt("")
	if !strings.Contains(prompt, "edge") || !strings.Contains(prompt, "本机主节点") {
		t.Fatalf("prompt %q", prompt)
	}
}
