package modules

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSchemaObject(t *testing.T) {
	var object map[string]any

	object = schemaObject(map[string]any{"type": "object", "properties": map[string]any{"a": 1}})
	if object["type"] != "object" || object["properties"] == nil {
		t.Fatalf("schemaObject(map) = %#v", object)
	}

	object = schemaObject(json.RawMessage(`{"properties":{"b":{"type":"string"}}}`))
	if object["type"] != "object" {
		t.Fatalf("schemaObject(raw) did not force object type: %#v", object)
	}

	object = schemaObject(nil)
	if object["type"] != "object" || object["properties"] == nil {
		t.Fatalf("schemaObject(nil) = %#v", object)
	}

	object = schemaObject("not a schema")
	if object["type"] != "object" {
		t.Fatalf("schemaObject(string) = %#v", object)
	}
}

func TestQualifiedName(t *testing.T) {
	var name string
	var repeated string

	name = qualifiedName(builtinServerName, "file_read")
	if name != "file_read" {
		t.Fatalf("builtin tool was renamed to %q", name)
	}

	name = qualifiedName("notion", "search")
	if name != "notion__search" {
		t.Fatalf("qualifiedName(notion, search) = %q", name)
	}

	name = qualifiedName("notion", "search pages/all")
	if name != "notion__search_pages_all" {
		t.Fatalf("qualifiedName did not sanitize: %q", name)
	}

	repeated = strings.Repeat("a", 90)
	name = qualifiedName("notion", repeated)
	if len(name) != maxToolNameLength {
		t.Fatalf("qualifiedName length = %d (%q)", len(name), name)
	}
	if name != qualifiedName("notion", repeated) {
		t.Fatal("qualifiedName truncation is not deterministic")
	}
	if name == qualifiedName("notion", repeated+"b") {
		t.Fatal("qualifiedName truncation collided")
	}
}

func TestAnnotationPermission(t *testing.T) {
	if annotationPermission(nil) != PermissionDangerous {
		t.Fatal("missing annotations must be dangerous")
	}
	if annotationPermission(&mcp.ToolAnnotations{ReadOnlyHint: true}) != PermissionSafe {
		t.Fatal("readOnlyHint must be safe")
	}
	if annotationPermission(&mcp.ToolAnnotations{ReadOnlyHint: false}) != PermissionDangerous {
		t.Fatal("writable tool must be dangerous")
	}
}

func TestOverridePermission(t *testing.T) {
	var entry MCPServer

	if overridePermission(&entry, "search", PermissionSafe) != PermissionSafe {
		t.Fatal("empty override must keep the derived permission")
	}

	entry.Permission = "dangerous"
	if overridePermission(&entry, "search", PermissionSafe) != PermissionDangerous {
		t.Fatal("server override was ignored")
	}

	entry.ToolPermission = map[string]string{"search": "safe"}
	if overridePermission(&entry, "search", PermissionDangerous) != PermissionSafe {
		t.Fatal("tool override did not win over the server override")
	}

	entry.ToolPermission = map[string]string{"search": "nonsense"}
	if overridePermission(&entry, "search", PermissionSafe) != PermissionDangerous {
		t.Fatal("invalid tool override must fall through to the server override")
	}
}

func TestResultText(t *testing.T) {
	var text string

	var err error

	text, err = resultText(&mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}})
	if err != nil || text != "hello" {
		t.Fatalf("resultText(text) = %q, %v", text, err)
	}

	text, err = resultText(&mcp.CallToolResult{StructuredContent: map[string]any{"ok": true}})
	if err != nil || text != `{"ok":true}` {
		t.Fatalf("resultText(structured) = %q, %v", text, err)
	}

	text, err = resultText(&mcp.CallToolResult{
		Content: []mcp.Content{&mcp.ImageContent{MIMEType: "image/png", Data: []byte("1234")}},
	})
	if err != nil || !strings.Contains(text, "image content omitted") || strings.Contains(text, "1234") {
		t.Fatalf("resultText(image) = %q, %v", text, err)
	}

	_, err = resultText(&mcp.CallToolResult{
		IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "boom"}},
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("resultText(error) = %v", err)
	}

	_, err = resultText(&mcp.CallToolResult{IsError: true})
	if err == nil || err.Error() != "tool call failed" {
		t.Fatalf("resultText(empty error) = %v", err)
	}
}
