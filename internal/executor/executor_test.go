package executor

import (
	"errors"
	"testing"
)

type fakeClient struct {
	response string
	err      error
	closed   bool
}

func (f *fakeClient) Execute(string) (string, error) {
	return f.response, f.err
}

func (f *fakeClient) Close() error {
	f.closed = true
	return nil
}

func TestNewExecutorRejectsEmptyPassword(t *testing.T) {
	if _, err := NewExecutor("127.0.0.1:25575", "  ", 5, true); !errors.Is(err, ErrPasswordEmpty) {
		t.Fatalf("expected ErrPasswordEmpty, got %v", err)
	}
}

func TestExecutorTrimsResponseAndCanUseNonEmptyErrorResponse(t *testing.T) {
	client := &fakeClient{response: "  server reply\n", err: errors.New("packet error")}
	executor := &Executor{client: client, skipErrors: true}

	response, err := executor.Execute("Info")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != "server reply" {
		t.Fatalf("unexpected response %q", response)
	}
	if err := executor.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !client.closed {
		t.Fatal("expected client to be closed")
	}
}
