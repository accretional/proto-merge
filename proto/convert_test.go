package proto

import (
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

func TestMessageToDescriptorProto(t *testing.T) {
	m := Message{Name: "Foo", Body: "message Foo { string bar = 1; }"}
	dp := MessageToDescriptorProto(m)
	if dp.GetName() != "Foo" {
		t.Errorf("expected Foo, got %s", dp.GetName())
	}
}

func TestServiceToDescriptorProto(t *testing.T) {
	s := Service{Name: "BarService", Body: "service BarService { rpc Do(A) returns(B); }"}
	dp := ServiceToDescriptorProto(s, "mypkg")
	if dp.GetName() != "BarService" {
		t.Errorf("expected BarService, got %s", dp.GetName())
	}
}

func TestEnumToDescriptorProto(t *testing.T) {
	e := Enum{Name: "Status", Body: "enum Status { UNKNOWN = 0; }"}
	dp := EnumToDescriptorProto(e)
	if dp.GetName() != "Status" {
		t.Errorf("expected Status, got %s", dp.GetName())
	}
}

func TestFileToDescriptorProto(t *testing.T) {
	f := &File{
		Syntax:  "proto3",
		Package: "test.v1",
		Imports: []string{"google/protobuf/empty.proto"},
		Messages: []Message{
			{Name: "Req"},
			{Name: "Resp"},
		},
		Services: []Service{
			{Name: "MySvc"},
		},
		Enums: []Enum{
			{Name: "Color"},
		},
	}
	fdp := FileToDescriptorProto(f, "test.proto")
	if fdp.GetName() != "test.proto" {
		t.Errorf("name = %s", fdp.GetName())
	}
	if fdp.GetPackage() != "test.v1" {
		t.Errorf("package = %s", fdp.GetPackage())
	}
	if fdp.GetSyntax() != "proto3" {
		t.Errorf("syntax = %s", fdp.GetSyntax())
	}
	if len(fdp.GetDependency()) != 1 {
		t.Errorf("deps = %d", len(fdp.GetDependency()))
	}
	if len(fdp.GetMessageType()) != 2 {
		t.Errorf("messages = %d", len(fdp.GetMessageType()))
	}
	if len(fdp.GetService()) != 1 {
		t.Errorf("services = %d", len(fdp.GetService()))
	}
	if len(fdp.GetEnumType()) != 1 {
		t.Errorf("enums = %d", len(fdp.GetEnumType()))
	}
}

func TestFileToDescriptorProto_EmptySyntax(t *testing.T) {
	f := &File{Package: "x"}
	fdp := FileToDescriptorProto(f, "x.proto")
	if fdp.GetSyntax() != "proto3" {
		t.Error("empty syntax should default to proto3")
	}
}

func TestParse_Empty(t *testing.T) {
	f, err := Parse("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Syntax != "" {
		t.Error("syntax should be empty")
	}
	if len(f.Messages) != 0 {
		t.Error("should have no messages")
	}
}

func TestParse_NestedMessage(t *testing.T) {
	src := `syntax = "proto3";
package nested;

message Outer {
  string name = 1;
  message Inner {
    int32 id = 1;
  }
  Inner child = 2;
}
`
	f, err := Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(f.Messages) != 1 {
		t.Fatalf("expected 1 top-level message, got %d", len(f.Messages))
	}
	if f.Messages[0].Name != "Outer" {
		t.Errorf("expected Outer, got %s", f.Messages[0].Name)
	}
	// Body should contain Inner
	if !containsStr(f.Messages[0].Body, "Inner") {
		t.Error("Outer body should contain Inner")
	}
}

func TestParse_MultipleServices(t *testing.T) {
	src := `syntax = "proto3";
package multi;

service A {
  rpc DoA(Req) returns(Resp);
}

service B {
  rpc DoB(Req) returns(Resp);
}
`
	f, err := Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(f.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(f.Services))
	}
}

func TestParse_MultipleEnums(t *testing.T) {
	src := `syntax = "proto3";
package enums;

enum Color {
  RED = 0;
  BLUE = 1;
}

enum Size {
  SMALL = 0;
  LARGE = 1;
}
`
	f, err := Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(f.Enums) != 2 {
		t.Errorf("expected 2 enums, got %d", len(f.Enums))
	}
}

func TestParse_MultipleImports(t *testing.T) {
	src := `syntax = "proto3";
import "a.proto";
import "b.proto";
import "c.proto";
`
	f, err := Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(f.Imports) != 3 {
		t.Errorf("expected 3 imports, got %d", len(f.Imports))
	}
}

func TestParse_MultipleOptions(t *testing.T) {
	src := `syntax = "proto3";
option go_package = "example.com/test";
option java_package = "com.example";
`
	f, err := Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(f.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(f.Options))
	}
}

func TestBundle_Empty(t *testing.T) {
	result := Bundle(nil)
	if result == "" {
		t.Error("bundle of nil should still produce header")
	}
}

func TestBundle_DuplicateNames(t *testing.T) {
	f1, _ := Parse(`syntax = "proto3"; package a;
message Foo { string x = 1; }`)
	f2, _ := Parse(`syntax = "proto3"; package b;
message Foo { int32 y = 1; }`)
	result := Bundle([]*File{f1, f2})
	// Should contain Foo only once (first wins)
	count := 0
	for i := 0; i < len(result)-3; i++ {
		if result[i:i+3] == "Foo" {
			count++
		}
	}
	// "Foo" appears in "message Foo {" — should be exactly 1
	if count != 1 {
		t.Errorf("expected Foo once in bundle, got %d occurrences", count)
	}
}

func TestSplit_EmptyPackage(t *testing.T) {
	f, _ := Parse(`syntax = "proto3";
message NoPackageMsg { string x = 1; }`)
	results := Split([]*File{f})
	for _, r := range results {
		if r.Dir != "default" {
			t.Errorf("expected dir 'default' for no-package file, got %s", r.Dir)
		}
	}
}

func TestSplit_DottedPackage(t *testing.T) {
	f, _ := Parse(`syntax = "proto3";
package com.example.v1;
message DotMsg { string x = 1; }`)
	results := Split([]*File{f})
	for _, r := range results {
		if r.Dir != "com/example/v1" {
			t.Errorf("expected dir 'com/example/v1', got %s", r.Dir)
		}
	}
}

func TestSplit_Deduplication(t *testing.T) {
	f1, _ := Parse(`syntax = "proto3"; package dup;
message Same { string x = 1; }`)
	f2, _ := Parse(`syntax = "proto3"; package dup;
message Same { string y = 1; }`)
	results := Split([]*File{f1, f2})
	count := 0
	for _, r := range results {
		if r.Filename == "same.proto" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 Same file, got %d", count)
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"HelloWorld", "hello_world"},
		{"HTTPRequest", "h_t_t_p_request"},
		{"already_snake", "already_snake"},
		{"A", "a"},
		{"", ""},
		{"URLParser", "u_r_l_parser"},
		{"simpleMsg", "simple_msg"},
	}
	for _, tt := range tests {
		got := toSnakeCase(tt.in)
		if got != tt.want {
			t.Errorf("toSnakeCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractBraceBlock_Unmatched(t *testing.T) {
	// Missing closing brace
	src := "{ hello world"
	result := extractBraceBlock(src, 0)
	if result != " hello world" {
		t.Errorf("expected fallback content, got %q", result)
	}
}

func TestExtractBraceBlock_OutOfBounds(t *testing.T) {
	result := extractBraceBlock("abc", 5)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestExtractBraceBlock_NotBrace(t *testing.T) {
	result := extractBraceBlock("abc", 0)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestExtractBraceBlock_Nested(t *testing.T) {
	src := "{ outer { inner } end }"
	result := extractBraceBlock(src, 0)
	if result != " outer { inner } end " {
		t.Errorf("got %q", result)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestParse_Proto2Syntax(t *testing.T) {
	src := `syntax = "proto2";
package legacy;
message OldMsg {
  required string name = 1;
}
`
	f, err := Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if f.Syntax != "proto2" {
		t.Errorf("expected proto2, got %s", f.Syntax)
	}
}

func TestParse_NoSyntax(t *testing.T) {
	src := `package nosyntax;
message Msg { string x = 1; }
`
	f, err := Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if f.Syntax != "" {
		t.Errorf("expected empty syntax, got %s", f.Syntax)
	}
}

func TestSplit_ServiceSuffix(t *testing.T) {
	f, _ := Parse(`syntax = "proto3"; package svc;
service MyService {
  rpc Do(Req) returns(Resp);
}`)
	results := Split([]*File{f})
	found := false
	for _, r := range results {
		if r.Filename == "my_service_service.proto" {
			found = true
		}
	}
	if !found {
		names := make([]string, len(results))
		for i, r := range results {
			names[i] = r.Filename
		}
		t.Errorf("expected my_service_service.proto, got %v", names)
	}
}

func TestParse_RealWorldProto(t *testing.T) {
	// Simulate a realistic proto from accretional/runrpc
	src := `syntax = "proto3";

package runner;

option go_package = "github.com/accretional/runrpc/runner";

import "google/protobuf/any.proto";
import "google/protobuf/empty.proto";

enum FileActionKind {
  FILE_ACTION_UNKNOWN = 0;
  FILE_ACTION_CREATE = 1;
  FILE_ACTION_DELETE = 2;
}

message Process {
  int32 pid = 1;
  string command = 2;
  repeated string args = 3;
  map<string, string> env = 4;
}

message SpawnRequest {
  string command = 1;
  repeated string args = 2;
}

message StopRequest {
  int32 pid = 1;
  bool force = 2;
}

message StopResponse {
  int32 exit_code = 1;
}

message StringValue {
  string value = 1;
}

service Runner {
  rpc Spawn(SpawnRequest) returns(Process);
  rpc Stop(StopRequest) returns(StopResponse);
  rpc List(google.protobuf.Empty) returns(stream Process);
}
`
	f, err := Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if f.Package != "runner" {
		t.Errorf("package = %s", f.Package)
	}
	if len(f.Imports) != 2 {
		t.Errorf("imports = %d, want 2", len(f.Imports))
	}
	if len(f.Messages) < 4 {
		t.Errorf("messages = %d, want >= 4", len(f.Messages))
	}
	if len(f.Services) != 1 {
		t.Errorf("services = %d, want 1", len(f.Services))
	}
	if f.Services[0].Name != "Runner" {
		t.Errorf("service name = %s", f.Services[0].Name)
	}
	if len(f.Enums) != 1 {
		t.Errorf("enums = %d, want 1", len(f.Enums))
	}
	if f.Enums[0].Name != "FileActionKind" {
		t.Errorf("enum name = %s", f.Enums[0].Name)
	}

	// Test that this parses through the full pipeline
	fdp := FileToDescriptorProto(f, "runner.proto")
	if len(fdp.GetMessageType()) < 4 {
		t.Errorf("fdp messages = %d", len(fdp.GetMessageType()))
	}

	// Split should produce separate files
	results := Split([]*File{f})
	if len(results) < 5 {
		t.Errorf("split produced %d files, want >= 5", len(results))
	}

	// All should be in runner/ directory
	for _, r := range results {
		if r.Dir != "runner" {
			t.Errorf("expected dir runner, got %s", r.Dir)
		}
	}

	// Bundle should include everything
	bundled := Bundle([]*File{f})
	for _, name := range []string{"Process", "SpawnRequest", "StopRequest", "Runner"} {
		if !containsSubstr(bundled, name) {
			t.Errorf("bundle missing %s", name)
		}
	}

	_ = descriptorpb.FieldDescriptorProto_TYPE_STRING // ensure import used
}
