package ai

import (
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// Registry: Register / Get / All / Names
// ---------------------------------------------------------------------------

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	tool := &Tool{Name: "test_tool", Description: "a test tool"}

	reg.Register(tool)

	got, ok := reg.Get("test_tool")
	if !ok {
		t.Fatal("Get returned ok=false; want true")
	}
	if got.Name != "test_tool" {
		t.Errorf("got.Name = %q; want %q", got.Name, "test_tool")
	}
	if got.Description != "a test tool" {
		t.Errorf("got.Description = %q; want %q", got.Description, "a test tool")
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	reg := NewRegistry()

	_, ok := reg.Get("nonexistent")
	if ok {
		t.Fatal("Get returned ok=true for nonexistent tool; want false")
	}
}

func TestRegistry_RegisterOverwrite(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Tool{Name: "t", Description: "first"})
	reg.Register(&Tool{Name: "t", Description: "second"})

	got, ok := reg.Get("t")
	if !ok {
		t.Fatal("Get returned ok=false")
	}
	if got.Description != "second" {
		t.Errorf("description = %q; want %q (overwrite)", got.Description, "second")
	}
}

func TestRegistry_All(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Tool{Name: "alpha"})
	reg.Register(&Tool{Name: "beta"})
	reg.Register(&Tool{Name: "gamma"})

	all := reg.All()
	if len(all) != 3 {
		t.Fatalf("All() returned %d tools; want 3", len(all))
	}

	names := make(map[string]bool)
	for _, tool := range all {
		names[tool.Name] = true
	}
	for _, expected := range []string{"alpha", "beta", "gamma"} {
		if !names[expected] {
			t.Errorf("All() missing tool %q", expected)
		}
	}
}

func TestRegistry_AllEmpty(t *testing.T) {
	reg := NewRegistry()
	all := reg.All()
	if len(all) != 0 {
		t.Fatalf("All() returned %d tools for empty registry; want 0", len(all))
	}
}

func TestRegistry_Names(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Tool{Name: "b_tool"})
	reg.Register(&Tool{Name: "a_tool"})

	names := reg.Names()
	if len(names) != 2 {
		t.Fatalf("Names() returned %d names; want 2", len(names))
	}

	sort.Strings(names)
	if names[0] != "a_tool" || names[1] != "b_tool" {
		t.Errorf("Names() = %v; want [a_tool b_tool]", names)
	}
}

func TestRegistry_NamesEmpty(t *testing.T) {
	reg := NewRegistry()
	names := reg.Names()
	if len(names) != 0 {
		t.Fatalf("Names() returned %d names for empty registry; want 0", len(names))
	}
}

// ---------------------------------------------------------------------------
// Helper: argString
// ---------------------------------------------------------------------------

func TestArgString_Present(t *testing.T) {
	args := map[string]any{"name": "Alice"}
	got := argString(args, "name")
	if got != "Alice" {
		t.Errorf("argString = %q; want %q", got, "Alice")
	}
}

func TestArgString_Missing(t *testing.T) {
	args := map[string]any{}
	got := argString(args, "name")
	if got != "" {
		t.Errorf("argString = %q; want empty string", got)
	}
}

func TestArgString_NonStringValue(t *testing.T) {
	args := map[string]any{"count": 42}
	got := argString(args, "count")
	if got != "42" {
		t.Errorf("argString = %q; want %q", got, "42")
	}
}

// ---------------------------------------------------------------------------
// Helper: argBool
// ---------------------------------------------------------------------------

func TestArgBool_True(t *testing.T) {
	args := map[string]any{"refresh": true}
	got := argBool(args, "refresh")
	if !got {
		t.Error("argBool = false; want true")
	}
}

func TestArgBool_False(t *testing.T) {
	args := map[string]any{"refresh": false}
	got := argBool(args, "refresh")
	if got {
		t.Error("argBool = true; want false")
	}
}

func TestArgBool_Missing(t *testing.T) {
	args := map[string]any{}
	got := argBool(args, "refresh")
	if got {
		t.Error("argBool = true for missing key; want false")
	}
}

func TestArgBool_NonBoolValue(t *testing.T) {
	args := map[string]any{"refresh": "yes"}
	got := argBool(args, "refresh")
	if got {
		t.Error("argBool = true for non-bool value; want false")
	}
}

// ---------------------------------------------------------------------------
// Helper: formatBytes
// ---------------------------------------------------------------------------

func TestFormatBytes_Zero(t *testing.T) {
	got := formatBytes(0)
	if got != "0 B" {
		t.Errorf("formatBytes(0) = %q; want %q", got, "0 B")
	}
}

func TestFormatBytes_Bytes(t *testing.T) {
	got := formatBytes(500)
	if got != "500.0 B" {
		t.Errorf("formatBytes(500) = %q; want %q", got, "500.0 B")
	}
}

func TestFormatBytes_Kilobytes(t *testing.T) {
	got := formatBytes(1024)
	if got != "1.0 KB" {
		t.Errorf("formatBytes(1024) = %q; want %q", got, "1.0 KB")
	}
}

func TestFormatBytes_Megabytes(t *testing.T) {
	got := formatBytes(1048576) // 1024*1024
	if got != "1.0 MB" {
		t.Errorf("formatBytes(1048576) = %q; want %q", got, "1.0 MB")
	}
}

func TestFormatBytes_Gigabytes(t *testing.T) {
	got := formatBytes(1073741824) // 1024^3
	if got != "1.0 GB" {
		t.Errorf("formatBytes(1073741824) = %q; want %q", got, "1.0 GB")
	}
}

func TestFormatBytes_LargeValue(t *testing.T) {
	// 2.5 GB
	got := formatBytes(2684354560)
	if got != "2.5 GB" {
		t.Errorf("formatBytes(2684354560) = %q; want %q", got, "2.5 GB")
	}
}

// ---------------------------------------------------------------------------
// Tool with Execute function
// ---------------------------------------------------------------------------

func TestTool_Execute(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Tool{
		Name:        "greet",
		Description: "greets the user",
		Parameters: []ToolParam{
			{Name: "name", Type: "string", Description: "name to greet", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			name := argString(args, "name")
			return "Hello, " + name + "!", nil
		},
	})

	tool, ok := reg.Get("greet")
	if !ok {
		t.Fatal("tool not found")
	}

	result, err := tool.Execute(map[string]any{"name": "World"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result != "Hello, World!" {
		t.Errorf("Execute result = %q; want %q", result, "Hello, World!")
	}
}
