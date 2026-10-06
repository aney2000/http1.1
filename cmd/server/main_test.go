package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func waitForServer(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get(url); err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server at %s did not start", url)
}

func TestRun_ServesUntilContextCancelled(t *testing.T) {
	addr := freeAddr(t)
	env := map[string]string{"ADDR": addr, "LOG_LEVEL": "error"}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, lookup, io.Discard) }()

	base := "http://" + addr
	waitForServer(t, base+"/health")

	// Use the standard library client as an independent interoperability check.
	resp, err := http.Post(base+"/notes", "application/json", strings.NewReader(`{"text":"hi"}`))
	if err != nil {
		t.Fatalf("POST /notes: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /notes status = %d", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() = %v, want nil after graceful shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return after cancel")
	}
}

func TestRun_InvalidConfigFails(t *testing.T) {
	lookup := func(k string) (string, bool) {
		if k == "READ_TIMEOUT" {
			return "never", true
		}
		return "", false
	}
	if err := run(context.Background(), lookup, io.Discard); err == nil {
		t.Fatal("run() = nil, want config error")
	}
}

func TestRun_ListenErrorIsReturned(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	lookup := func(k string) (string, bool) {
		if k == "ADDR" {
			return ln.Addr().String(), true
		}
		return "", false
	}

	if err := run(context.Background(), lookup, io.Discard); err == nil {
		t.Fatal("run() = nil, want address-in-use error")
	}
}
