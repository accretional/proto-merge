package walk

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func nestedFileDescriptor() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("nested.proto"),
		Package: proto.String("nested"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Outer"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("name"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
				NestedType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("Inner"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   proto.String("id"),
								Number: proto.Int32(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
								Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							},
						},
						NestedType: []*descriptorpb.DescriptorProto{
							{
								Name: proto.String("DeepNested"),
								Field: []*descriptorpb.FieldDescriptorProto{
									{
										Name:   proto.String("flag"),
										Number: proto.Int32(1),
										Type:   descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(),
										Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func multiServiceFileDescriptor() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("multi.proto"),
		Package: proto.String("multi"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Req"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("query"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
			{
				Name: proto.String("Resp"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("result"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("Alpha"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("DoAlpha"),
						InputType:  proto.String(".multi.Req"),
						OutputType: proto.String(".multi.Resp"),
					},
				},
			},
			{
				Name: proto.String("Beta"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("DoBeta"),
						InputType:  proto.String(".multi.Req"),
						OutputType: proto.String(".multi.Resp"),
					},
				},
			},
		},
	}
}

func TestWalkMessages_Nested(t *testing.T) {
	fdp := nestedFileDescriptor()
	var visited []string
	err := WalkMessages(fdp, func(path string, msg *descriptorpb.DescriptorProto) error {
		visited = append(visited, path)
		return nil
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Should visit Outer, Outer.Inner, Outer.Inner.DeepNested
	if len(visited) != 3 {
		t.Errorf("expected 3 messages, got %d: %v", len(visited), visited)
	}
	expected := []string{"Outer", "Outer.Inner", "Outer.Inner.DeepNested"}
	for i, e := range expected {
		if i >= len(visited) || visited[i] != e {
			t.Errorf("visited[%d] = %q, want %q", i, visited[i], e)
		}
	}
}

func TestWalkMessages_VisitorError(t *testing.T) {
	fdp := testFileDescriptor()
	err := WalkMessages(fdp, func(path string, msg *descriptorpb.DescriptorProto) error {
		return fmt.Errorf("stop here")
	})
	if err == nil {
		t.Error("expected error from visitor")
	}
}

func TestWalkServices_VisitorError(t *testing.T) {
	fdp := testFileDescriptor()
	err := WalkServices(fdp, func(svc *descriptorpb.ServiceDescriptorProto) error {
		return fmt.Errorf("stop here")
	})
	if err == nil {
		t.Error("expected error from visitor")
	}
}

func TestWalk_NestedMessageLogging(t *testing.T) {
	fdp := nestedFileDescriptor()
	result, err := Walk(fdp, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Log should contain entries for nested messages
	hasInner := false
	hasDeep := false
	for _, entry := range result.Log {
		if containsStr(entry, "Inner") {
			hasInner = true
		}
		if containsStr(entry, "DeepNested") {
			hasDeep = true
		}
	}
	if !hasInner {
		t.Error("log missing Inner message visit")
	}
	if !hasDeep {
		t.Error("log missing DeepNested message visit")
	}
}

func TestWalk_MultipleServices(t *testing.T) {
	fdp := multiServiceFileDescriptor()
	result, err := Walk(fdp, []TransformCommand{
		{
			AddMethod: &ServiceModification{
				MethodName: "HealthCheck",
				InputType:  "Req",
				OutputType: "Resp",
			},
		},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Both services should have HealthCheck
	for _, svc := range result.File.GetService() {
		found := false
		for _, m := range svc.GetMethod() {
			if m.GetName() == "HealthCheck" {
				found = true
			}
		}
		if !found {
			t.Errorf("HealthCheck not added to %s", svc.GetName())
		}
	}
}

func TestWalk_EmptyFile(t *testing.T) {
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("empty.proto"),
		Package: proto.String("empty"),
		Syntax:  proto.String("proto3"),
	}
	result, err := Walk(fdp, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.File.GetName() != "empty.proto" {
		t.Error("wrong file name")
	}
}

func TestWalk_RemoveThenAdd(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := Walk(fdp, []TransformCommand{
		{
			RemoveField: &RemoveField{
				MessageName: "UserRequest",
				FieldName:   "name",
			},
		},
		{
			AddField: &FieldModification{
				FieldName:     "full_name",
				FieldType:     descriptorpb.FieldDescriptorProto_TYPE_STRING,
				TargetMessage: "UserRequest",
			},
		},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	for _, m := range result.File.GetMessageType() {
		if m.GetName() == "UserRequest" {
			hasName := false
			hasFullName := false
			for _, f := range m.GetField() {
				if f.GetName() == "name" {
					hasName = true
				}
				if f.GetName() == "full_name" {
					hasFullName = true
				}
			}
			if hasName {
				t.Error("name should have been removed")
			}
			if !hasFullName {
				t.Error("full_name should have been added")
			}
		}
	}
}

func TestWalk_InvalidCommand(t *testing.T) {
	fdp := testFileDescriptor()
	// An empty command (no fields set) should be a no-op
	result, err := Walk(fdp, []TransformCommand{{}})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// File should be unchanged
	if len(result.File.GetMessageType()) != 2 {
		t.Error("file should be unchanged")
	}
}

func TestWalk_DoesNotMutateInput(t *testing.T) {
	fdp := testFileDescriptor()
	originalMsgCount := len(fdp.GetMessageType())
	originalFieldCount := len(fdp.GetMessageType()[0].GetField())

	_, err := Walk(fdp, []TransformCommand{
		{
			AddField: &FieldModification{
				FieldName: "new_field",
				FieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING,
			},
		},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Original should be untouched
	if len(fdp.GetMessageType()) != originalMsgCount {
		t.Error("input message count was mutated")
	}
	if len(fdp.GetMessageType()[0].GetField()) != originalFieldCount {
		t.Error("input field count was mutated")
	}
}

func TestWalk_CommandFailure(t *testing.T) {
	fdp := testFileDescriptor()
	// Try to merge with a non-existent source — should fail
	_, err := Walk(fdp, []TransformCommand{
		{
			MergeMessages: &MergeMessages{
				SourceMessage: "NonExistent",
				TargetMessage: "UserResponse",
			},
		},
	})
	if err == nil {
		t.Error("expected error for invalid merge")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
