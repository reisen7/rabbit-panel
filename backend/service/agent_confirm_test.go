package service

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConfirmStoreApproveReject(t *testing.T) {
	store := newConfirmStore()
	p, err := store.add("delete_container(abc)")
	if err != nil {
		t.Fatal(err)
	}
	id := p.id

	done := make(chan bool, 1)
	go func() {
		done <- store.wait(context.Background(), p, time.Minute)
	}()

	// 未知 ID 不能响应
	if store.resolve("nonexistent", true) {
		t.Fatal("unknown id must not resolve")
	}

	if !store.resolve(id, true) {
		t.Fatal("expected resolve to succeed")
	}
	if approved := <-done; !approved {
		t.Fatal("expected approval")
	}

	// 已解决的不能再次响应
	if store.resolve(id, true) {
		t.Fatal("double resolve must fail")
	}
}

func TestConfirmStoreTimeout(t *testing.T) {
	store := newConfirmStore()
	p, _ := store.add("prune_images()")
	if store.wait(context.Background(), p, 50*time.Millisecond) {
		t.Fatal("expected timeout to count as rejection")
	}
}

func TestConfirmStoreCancel(t *testing.T) {
	store := newConfirmStore()
	p, _ := store.add("stop_container(x)")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if store.wait(ctx, p, time.Minute) {
		t.Fatal("expected ctx cancellation to count as rejection")
	}
}

func TestNeedsConfirmClassification(t *testing.T) {
	dangerous := []string{"stop_container", "restart_container", "delete_container", "delete_image",
		"delete_network", "delete_volume", "compose_down", "compose_restart", "prune_containers", "prune_images"}
	for _, name := range dangerous {
		if !needsConfirm(name) {
			t.Errorf("%s must require confirmation", name)
		}
	}
	safe := []string{"list_containers", "inspect_container", "get_container_logs", "system_status",
		"list_nodes", "compose_status", "list_images", "compose_up", "run_container"}
	for _, name := range safe {
		if needsConfirm(name) {
			t.Errorf("%s must not require confirmation", name)
		}
	}
}

// 非交互通道（微信）：危险操作必须被拒绝且不执行
func TestExecuteToolWithConfirmNonInteractiveRejects(t *testing.T) {
	svc := &AgentService{confirms: newConfirmStore()}
	result, head := svc.executeToolWithConfirm(context.Background(), "delete_container(abc123)", func(ChatEvent) error {
		return nil
	}, false)
	if !strings.Contains(result, "已拒绝执行") {
		t.Fatalf("expected rejection, got %q", result)
	}
	if !strings.Contains(head, "拒绝") {
		t.Fatalf("expected rejection head, got %q", head)
	}
}

// stopStubDocker 记录收到的 stop 调用，用于验证批准后确实执行
type stopStubDocker struct {
	listOnlyDocker
	mu      sync.Mutex
	stopped []string
}

func (d *stopStubDocker) ContainerStop(_ context.Context, id string, _ *int) error {
	d.mu.Lock()
	d.stopped = append(d.stopped, id)
	d.mu.Unlock()
	return nil
}

// 交互通道：批准后执行；事件流必须先收到 confirm 事件
func TestExecuteToolWithConfirmInteractiveApprove(t *testing.T) {
	dockerStub := &stopStubDocker{}
	svc := &AgentService{confirms: newConfirmStore(), dockerRepo: dockerStub}

	var mu sync.Mutex
	var events []ChatEvent
	emit := func(evt ChatEvent) error {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
		return nil
	}

	type outcome struct {
		result string
		head   string
	}
	done := make(chan outcome, 1)
	go func() {
		r, h := svc.executeToolWithConfirm(context.Background(), "stop_container(abc)", emit, true)
		done <- outcome{r, h}
	}()

	// 等 confirm 事件出现
	var confirmID string
	for i := 0; i < 200; i++ {
		mu.Lock()
		for _, evt := range events {
			if evt.Type == "confirm" {
				confirmID = evt.ID
			}
		}
		mu.Unlock()
		if confirmID != "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if confirmID == "" {
		t.Fatal("no confirm event emitted")
	}

	if !svc.ResolveConfirm(confirmID, true) {
		t.Fatal("resolve failed")
	}

	select {
	case out := <-done:
		if strings.Contains(out.result, "未执行") || strings.Contains(out.result, "拒绝") {
			t.Fatalf("approved operation must execute, got %q", out.result)
		}
		if !strings.Contains(out.result, "stopped") {
			t.Fatalf("expected stop result, got %q", out.result)
		}
		dockerStub.mu.Lock()
		n := len(dockerStub.stopped)
		dockerStub.mu.Unlock()
		if n != 1 {
			t.Fatalf("expected exactly one ContainerStop call, got %d", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("execution did not proceed after approval")
	}
}

// 交互通道：拒绝后不执行
func TestExecuteToolWithConfirmInteractiveReject(t *testing.T) {
	svc := &AgentService{confirms: newConfirmStore()}

	var events []ChatEvent
	var mu sync.Mutex
	emit := func(evt ChatEvent) error {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
		return nil
	}

	done := make(chan string, 1)
	go func() {
		r, _ := svc.executeToolWithConfirm(context.Background(), "prune_containers()", emit, true)
		done <- r
	}()

	var confirmID string
	for i := 0; i < 200; i++ {
		mu.Lock()
		for _, evt := range events {
			if evt.Type == "confirm" {
				confirmID = evt.ID
			}
		}
		mu.Unlock()
		if confirmID != "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if confirmID == "" {
		t.Fatal("no confirm event emitted")
	}
	svc.ResolveConfirm(confirmID, false)

	select {
	case r := <-done:
		if !strings.Contains(r, "未执行") {
			t.Fatalf("expected rejection result, got %q", r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no result after rejection")
	}
}
