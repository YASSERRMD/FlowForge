package actions

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestRegistryExecuteEcho(t *testing.T) {
	r := Default()
	out, err := r.Execute(context.Background(), "mock.echo", json.RawMessage(`{"value":"hello"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var decoded EchoOutput
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !decoded.OK || string(decoded.Echo) != `{"value":"hello"}` {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestEchoIsDeterministic(t *testing.T) {
	r := Default()
	input := json.RawMessage(`{"a":1,"b":[1,2,3]}`)
	first, err := r.Execute(context.Background(), "mock.echo", input)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		again, err := r.Execute(context.Background(), "mock.echo", input)
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(again) {
			t.Fatalf("echo not deterministic: %s vs %s", first, again)
		}
	}
}

func TestRegistryRejectsUnknownType(t *testing.T) {
	r := Default()
	_, err := r.Execute(context.Background(), "mock.nope", nil)
	if !errors.Is(err, ErrUnknownType) {
		t.Fatalf("expected ErrUnknownType, got %v", err)
	}
}

func TestFailActionAlwaysFails(t *testing.T) {
	r := Default()
	_, err := r.Execute(context.Background(), "mock.fail",
		json.RawMessage(`{"message":"synthetic failure"}`))
	if err == nil || err.Error() != "synthetic failure" {
		t.Fatalf("expected configured failure, got %v", err)
	}
}

func TestEchoRejectsNonObject(t *testing.T) {
	r := Default()
	if _, err := r.Execute(context.Background(), "mock.echo", json.RawMessage(`[1,2]`)); err == nil {
		t.Fatal("expected array input to be rejected")
	}
}

func TestDuplicateRegistrationPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	r := NewRegistry()
	r.Register(Echo{})
	r.Register(Echo{})
}

func TestTypesAreSorted(t *testing.T) {
	types := Default().Types()
	if len(types) != 2 || types[0] != "mock.echo" || types[1] != "mock.fail" {
		t.Fatalf("types = %v", types)
	}
}
