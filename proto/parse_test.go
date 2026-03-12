package proto

import (
	"strings"
	"testing"
)

const testProto = `syntax = "proto3";

package example;

option go_package = "example.com/test";

import "google/protobuf/empty.proto";

enum Status {
  UNKNOWN = 0;
  ACTIVE = 1;
}

message HelloRequest {
  string name = 1;
  int32 count = 2;
}

message HelloResponse {
  string greeting = 1;
}

service Greeter {
  rpc SayHello(HelloRequest) returns(HelloResponse);
  rpc SayGoodbye(HelloRequest) returns(google.protobuf.Empty);
}
`

func TestParse(t *testing.T) {
	f, err := Parse(testProto)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if f.Syntax != "proto3" {
		t.Errorf("syntax = %q, want proto3", f.Syntax)
	}
	if f.Package != "example" {
		t.Errorf("package = %q, want example", f.Package)
	}
	if len(f.Imports) != 1 || f.Imports[0] != "google/protobuf/empty.proto" {
		t.Errorf("imports = %v, want [google/protobuf/empty.proto]", f.Imports)
	}
	if len(f.Messages) != 2 {
		t.Errorf("got %d messages, want 2", len(f.Messages))
	} else {
		if f.Messages[0].Name != "HelloRequest" {
			t.Errorf("message[0] = %q, want HelloRequest", f.Messages[0].Name)
		}
		if f.Messages[1].Name != "HelloResponse" {
			t.Errorf("message[1] = %q, want HelloResponse", f.Messages[1].Name)
		}
	}
	if len(f.Services) != 1 || f.Services[0].Name != "Greeter" {
		t.Errorf("services = %v, want [Greeter]", f.Services)
	}
	if len(f.Enums) != 1 || f.Enums[0].Name != "Status" {
		t.Errorf("enums = %v, want [Status]", f.Enums)
	}
}

func TestBundle(t *testing.T) {
	f1, _ := Parse(testProto)
	f2, _ := Parse(`syntax = "proto3";
package other;

message ExtraMsg {
  bool flag = 1;
}
`)
	bundled := Bundle([]*File{f1, f2})

	if !strings.Contains(bundled, "HelloRequest") {
		t.Error("bundle missing HelloRequest")
	}
	if !strings.Contains(bundled, "ExtraMsg") {
		t.Error("bundle missing ExtraMsg")
	}
	if !strings.Contains(bundled, "Greeter") {
		t.Error("bundle missing Greeter service")
	}
	if !strings.Contains(bundled, `package bundle`) {
		t.Error("bundle missing package declaration")
	}
}

func TestSplit(t *testing.T) {
	f, _ := Parse(testProto)
	results := Split([]*File{f})

	if len(results) == 0 {
		t.Fatal("split produced no results")
	}

	// Should have: Status enum, HelloRequest, HelloResponse, Greeter service
	names := map[string]bool{}
	for _, r := range results {
		names[r.Filename] = true
		if r.Dir != "example" {
			t.Errorf("expected dir 'example', got %q", r.Dir)
		}
		if !strings.Contains(r.Content, "package example") {
			t.Errorf("file %s missing package declaration", r.Filename)
		}
	}

	expected := []string{"hello_request.proto", "hello_response.proto", "greeter_service.proto", "status.proto"}
	for _, e := range expected {
		if !names[e] {
			t.Errorf("missing expected file %q, got %v", e, names)
		}
	}
}
