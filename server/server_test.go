package server

import (
	"context"
	"strings"
	"testing"

	"github.com/accretional/merge/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func testFileDescriptor() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Request"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("name"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
			{
				Name: proto.String("Response"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("id"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("TestService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("Get"),
						InputType:  proto.String(".test.Request"),
						OutputType: proto.String(".test.Response"),
					},
				},
			},
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("Status"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("UNKNOWN"), Number: proto.Int32(0)},
				},
			},
		},
	}
}

func TestDescribe_File(t *testing.T) {
	srv := New("")
	resp, err := srv.Describe(context.Background(), &pb.Descriptor{
		Kind: &pb.Descriptor_File{File: testFileDescriptor()},
	})
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}
	if !strings.Contains(resp.GetValue(), "test.proto") {
		t.Error("missing file name in description")
	}
	if !strings.Contains(resp.GetValue(), "messages: 2") {
		t.Error("wrong message count")
	}
}

func TestDescribe_Message(t *testing.T) {
	srv := New("")
	resp, err := srv.Describe(context.Background(), &pb.Descriptor{
		Kind: &pb.Descriptor_Message{Message: testFileDescriptor().GetMessageType()[0]},
	})
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}
	if !strings.Contains(resp.GetValue(), "Request") {
		t.Error("missing message name")
	}
}

func TestDescribe_Service(t *testing.T) {
	srv := New("")
	resp, err := srv.Describe(context.Background(), &pb.Descriptor{
		Kind: &pb.Descriptor_Service{Service: testFileDescriptor().GetService()[0]},
	})
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}
	if !strings.Contains(resp.GetValue(), "TestService") {
		t.Error("missing service name")
	}
}

func TestDescribe_Method(t *testing.T) {
	srv := New("")
	resp, err := srv.Describe(context.Background(), &pb.Descriptor{
		Kind: &pb.Descriptor_Method{Method: testFileDescriptor().GetService()[0].GetMethod()[0]},
	})
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}
	if !strings.Contains(resp.GetValue(), "Get") {
		t.Error("missing method name")
	}
}

func TestDescribe_Field(t *testing.T) {
	srv := New("")
	resp, err := srv.Describe(context.Background(), &pb.Descriptor{
		Kind: &pb.Descriptor_Field{Field: testFileDescriptor().GetMessageType()[0].GetField()[0]},
	})
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}
	if !strings.Contains(resp.GetValue(), "name") {
		t.Error("missing field name")
	}
}

func TestDescribe_Enum(t *testing.T) {
	srv := New("")
	resp, err := srv.Describe(context.Background(), &pb.Descriptor{
		Kind: &pb.Descriptor_Enum{Enum: testFileDescriptor().GetEnumType()[0]},
	})
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}
	if !strings.Contains(resp.GetValue(), "Status") {
		t.Error("missing enum name")
	}
}

func TestDescribe_Empty(t *testing.T) {
	srv := New("")
	_, err := srv.Describe(context.Background(), &pb.Descriptor{})
	if err == nil {
		t.Error("expected error for empty descriptor")
	}
}

func TestTransform_AddUUID(t *testing.T) {
	srv := New("")
	resp, err := srv.Transform(context.Background(), &pb.WalkRequest{
		File: testFileDescriptor(),
		Commands: []*pb.TransformCommand{
			{
				Command: &pb.TransformCommand_AddField{
					AddField: &pb.FieldModification{
						FieldName: "uuid",
						FieldType: int32(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	for _, m := range resp.GetFile().GetMessageType() {
		found := false
		for _, f := range m.GetField() {
			if f.GetName() == "uuid" {
				found = true
			}
		}
		if !found {
			t.Errorf("uuid not found in %s", m.GetName())
		}
	}

	if len(resp.GetLog()) == 0 {
		t.Error("expected log entries")
	}
}

func TestTransform_AddMethod(t *testing.T) {
	srv := New("")
	resp, err := srv.Transform(context.Background(), &pb.WalkRequest{
		File: testFileDescriptor(),
		Commands: []*pb.TransformCommand{
			{
				Command: &pb.TransformCommand_AddMethod{
					AddMethod: &pb.ServiceModification{
						MethodName: "Ping",
						InputType:  "Request",
						OutputType: "Response",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	svc := resp.GetFile().GetService()[0]
	found := false
	for _, m := range svc.GetMethod() {
		if m.GetName() == "Ping" {
			found = true
		}
	}
	if !found {
		t.Error("Ping method not added")
	}
}

func TestTransform_Rename(t *testing.T) {
	srv := New("")
	resp, err := srv.Transform(context.Background(), &pb.WalkRequest{
		File: testFileDescriptor(),
		Commands: []*pb.TransformCommand{
			{
				Command: &pb.TransformCommand_Rename{
					Rename: &pb.RenameSymbol{
						OldName: "Request",
						NewName: "AccountRequest",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	found := false
	for _, m := range resp.GetFile().GetMessageType() {
		if m.GetName() == "AccountRequest" {
			found = true
		}
		if m.GetName() == "Request" {
			t.Error("Request should have been renamed")
		}
	}
	if !found {
		t.Error("AccountRequest not found")
	}
}

func TestTransform_MergeMessages(t *testing.T) {
	srv := New("")
	resp, err := srv.Transform(context.Background(), &pb.WalkRequest{
		File: testFileDescriptor(),
		Commands: []*pb.TransformCommand{
			{
				Command: &pb.TransformCommand_MergeMessages{
					MergeMessages: &pb.MergeMessages{
						SourceMessage: "Request",
						TargetMessage: "Response",
						ResultName:    "Combined",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	var combined *descriptorpb.DescriptorProto
	for _, m := range resp.GetFile().GetMessageType() {
		if m.GetName() == "Combined" {
			combined = m
		}
	}
	if combined == nil {
		t.Fatal("Combined message not found")
	}

	fields := map[string]bool{}
	for _, f := range combined.GetField() {
		fields[f.GetName()] = true
	}
	if !fields["id"] || !fields["name"] {
		t.Errorf("expected id and name fields, got %v", fields)
	}
}

func TestTransform_RemoveField(t *testing.T) {
	srv := New("")
	resp, err := srv.Transform(context.Background(), &pb.WalkRequest{
		File: testFileDescriptor(),
		Commands: []*pb.TransformCommand{
			{
				Command: &pb.TransformCommand_RemoveField{
					RemoveField: &pb.RemoveField{
						MessageName: "Request",
						FieldName:   "name",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	for _, m := range resp.GetFile().GetMessageType() {
		if m.GetName() == "Request" {
			for _, f := range m.GetField() {
				if f.GetName() == "name" {
					t.Error("name field should have been removed")
				}
			}
		}
	}
}

func TestTransform_ChainedCommands(t *testing.T) {
	srv := New("")
	resp, err := srv.Transform(context.Background(), &pb.WalkRequest{
		File: testFileDescriptor(),
		Commands: []*pb.TransformCommand{
			{
				Command: &pb.TransformCommand_AddField{
					AddField: &pb.FieldModification{
						FieldName: "uuid",
						FieldType: int32(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					},
				},
			},
			{
				Command: &pb.TransformCommand_AddField{
					AddField: &pb.FieldModification{
						FieldName: "created_at",
						FieldType: int32(descriptorpb.FieldDescriptorProto_TYPE_INT64),
					},
				},
			},
			{
				Command: &pb.TransformCommand_AddMethod{
					AddMethod: &pb.ServiceModification{
						MethodName: "Identify",
						InputType:  "Request",
						OutputType: "Response",
					},
				},
			},
			{
				Command: &pb.TransformCommand_AddMethod{
					AddMethod: &pb.ServiceModification{
						MethodName: "Discover",
						InputType:  "Request",
						OutputType: "Response",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Check all messages have both new fields
	for _, m := range resp.GetFile().GetMessageType() {
		fields := map[string]bool{}
		for _, f := range m.GetField() {
			fields[f.GetName()] = true
		}
		if !fields["uuid"] {
			t.Errorf("uuid missing from %s", m.GetName())
		}
		if !fields["created_at"] {
			t.Errorf("created_at missing from %s", m.GetName())
		}
	}

	// Check service has new methods
	svc := resp.GetFile().GetService()[0]
	methods := map[string]bool{}
	for _, m := range svc.GetMethod() {
		methods[m.GetName()] = true
	}
	for _, expected := range []string{"Get", "Identify", "Discover"} {
		if !methods[expected] {
			t.Errorf("missing method %s", expected)
		}
	}
}

func TestTransform_NilFile(t *testing.T) {
	srv := New("")
	_, err := srv.Transform(context.Background(), &pb.WalkRequest{})
	if err == nil {
		t.Error("expected error for nil file")
	}
}
