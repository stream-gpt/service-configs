package configclient

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	c := newCache(time.Minute)

	val := json.RawMessage(`"hello"`)
	c.Set("key1", val)

	got, found := c.Get("key1")
	if !found {
		t.Fatal("expected key1 to be found")
	}
	if string(got) != `"hello"` {
		t.Fatalf("expected %q, got %q", `"hello"`, string(got))
	}
}

func TestCache_GetNotFound(t *testing.T) {
	c := newCache(time.Minute)

	_, found := c.Get("missing")
	if found {
		t.Fatal("expected missing key to not be found")
	}
}

func TestCache_IsExpired(t *testing.T) {
	c := newCache(time.Millisecond)

	c.Set("key1", json.RawMessage(`1`))
	time.Sleep(5 * time.Millisecond)

	if !c.IsExpired("key1") {
		t.Fatal("expected key1 to be expired")
	}

	// Value should still be accessible (for graceful degradation)
	val, found := c.Get("key1")
	if !found {
		t.Fatal("expected expired key1 to still be found in cache")
	}
	if string(val) != `1` {
		t.Fatalf("expected %q, got %q", `1`, string(val))
	}
}

func TestCache_SetBatch(t *testing.T) {
	c := newCache(time.Minute)

	c.SetBatch(map[string]json.RawMessage{
		"a": json.RawMessage(`"alpha"`),
		"b": json.RawMessage(`"beta"`),
	})

	for _, key := range []string{"a", "b"} {
		if _, found := c.Get(key); !found {
			t.Fatalf("expected key %q to be found", key)
		}
	}
}
