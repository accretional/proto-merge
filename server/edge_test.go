package server

import (
	"context"
	"strings"
	"testing"

	"github.com/accretional/merge/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestPbCommandsToWalk_AllTypes(t *testing.T) {
	cmds := []*pb.TransformCommand{
		{Command: &pb.TransformCommand_AddField{
			AddField: &pb.FieldModification{
				FieldName:     "f1",
				FieldType:     int32(descriptorpb.FieldDescriptorProto_TYPE_STRING),
				TypeName:      "SomeType",
				FieldNumber:   5,
				Repeated:      true,
				TargetMessage: "Msg",
			},
		}},
		{Command: &pb.TransformCommand_AddMethod{
			AddMethod: &pb.ServiceModification{
				MethodName:     "Rpc1",
				InputType:      "In",
				OutputType:     "Out",
				ClientStreaming: true,
				ServerStreaming: true,
				TargetService:  "Svc",
			},
		}},
		{Command: &pb.TransformCommand_MergeMessages{
			MergeMessages: &pb.MergeMessages{
				SourceMessage: "A",
				TargetMessage: "B",
				ResultName:    "C",
			},
		}},
		{Command: &pb.TransformCommand_Rename{
			Rename: &pb.RenameSymbol{
				OldName: "old",
				NewName: "new",
			},
		}},
		{Command: &pb.TransformCommand_RemoveField{
			RemoveField: &pb.RemoveField{
				MessageName: "Msg",
				FieldName:   "f",
			},
		}},
	}

	result := pbCommandsToWalk(cmds)
	if len(result) != 5 {
		t.Fatalf("expected 5 commands, got %d", len(result))
	}

	// Verify AddField
	af := result[0].AddField
	if af == nil {
		t.Fatal("AddField nil")
	}
	if af.FieldName != "f1" || af.TypeName != "SomeType" || af.FieldNumber != 5 || !af.Repeated || af.TargetMessage != "Msg" {
		t.Error("AddField fields mismatch")
	}

	// Verify AddMethod
	am := result[1].AddMethod
	if am == nil {
		t.Fatal("AddMethod nil")
	}
	if am.MethodName != "Rpc1" || !am.ClientStreaming || !am.ServerStreaming || am.TargetService != "Svc" {
		t.Error("AddMethod fields mismatch")
	}

	// Verify MergeMessages
	mm := result[2].MergeMessages
	if mm == nil {
		t.Fatal("MergeMessages nil")
	}
	if mm.SourceMessage != "A" || mm.TargetMessage != "B" || mm.ResultName != "C" {
		t.Error("MergeMessages fields mismatch")
	}

	// Verify Rename
	rn := result[3].Rename
	if rn == nil {
		t.Fatal("Rename nil")
	}
	if rn.OldName != "old" || rn.NewName != "new" {
		t.Error("Rename fields mismatch")
	}

	// Verify RemoveField
	rf := result[4].RemoveField
	if rf == nil {
		t.Fatal("RemoveField nil")
	}
	if rf.MessageName != "Msg" || rf.FieldName != "f" {
		t.Error("RemoveField fields mismatch")
	}
}

func TestPbCommandsToWalk_Empty(t *testing.T) {
	result := pbCommandsToWalk(nil)
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestTransform_EmptyCommands(t *testing.T) {
	srv := New("")
	resp, err := srv.Transform(context.Background(), &pb.WalkRequest{
		File:     testFileDescriptor(),
		Commands: nil,
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// File should be unchanged
	if len(resp.GetFile().GetMessageType()) != 2 {
		t.Error("file should be unchanged")
	}
}

func TestTransform_MultipleFieldTypes(t *testing.T) {
	types := []struct {
		name    string
		fdpType descriptorpb.FieldDescriptorProto_Type
	}{
		{"int32_field", descriptorpb.FieldDescriptorProto_TYPE_INT32},
		{"int64_field", descriptorpb.FieldDescriptorProto_TYPE_INT64},
		{"bool_field", descriptorpb.FieldDescriptorProto_TYPE_BOOL},
		{"bytes_field", descriptorpb.FieldDescriptorProto_TYPE_BYTES},
		{"double_field", descriptorpb.FieldDescriptorProto_TYPE_DOUBLE},
	}

	for _, tt := range types {
		srv := New("")
		resp, err := srv.Transform(context.Background(), &pb.WalkRequest{
			File: testFileDescriptor(),
			Commands: []*pb.TransformCommand{
				{Command: &pb.TransformCommand_AddField{
					AddField: &pb.FieldModification{
						FieldName:     tt.name,
						FieldType:     int32(tt.fdpType),
						TargetMessage: "Request",
					},
				}},
			},
		})
		if err != nil {
			t.Errorf("error adding %s: %v", tt.name, err)
			continue
		}
		found := false
		for _, m := range resp.GetFile().GetMessageType() {
			if m.GetName() == "Request" {
				for _, f := range m.GetField() {
					if f.GetName() == tt.name {
						found = true
					}
				}
			}
		}
		if !found {
			t.Errorf("%s not added to Request", tt.name)
		}
	}
}

func TestTransform_RemoveAndAdd(t *testing.T) {
	srv := New("")
	resp, err := srv.Transform(context.Background(), &pb.WalkRequest{
		File: testFileDescriptor(),
		Commands: []*pb.TransformCommand{
			{Command: &pb.TransformCommand_RemoveField{
				RemoveField: &pb.RemoveField{
					MessageName: "Request",
					FieldName:   "name",
				},
			}},
			{Command: &pb.TransformCommand_AddField{
				AddField: &pb.FieldModification{
					FieldName:     "display_name",
					FieldType:     int32(descriptorpb.FieldDescriptorProto_TYPE_STRING),
					TargetMessage: "Request",
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	for _, m := range resp.GetFile().GetMessageType() {
		if m.GetName() == "Request" {
			fields := map[string]bool{}
			for _, f := range m.GetField() {
				fields[f.GetName()] = true
			}
			if fields["name"] {
				t.Error("name should be removed")
			}
			if !fields["display_name"] {
				t.Error("display_name should be added")
			}
		}
	}
}

func TestDescribe_FileWithEnumsAndServices(t *testing.T) {
	srv := New("")
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("full.proto"),
		Package: proto.String("full"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Msg1")},
			{Name: proto.String("Msg2")},
			{Name: proto.String("Msg3")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{Name: proto.String("Svc1")},
			{Name: proto.String("Svc2")},
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{Name: proto.String("E1"), Value: []*descriptorpb.EnumValueDescriptorProto{
				{Name: proto.String("UNKNOWN"), Number: proto.Int32(0)},
			}},
		},
	}
	resp, err := srv.Describe(context.Background(), &pb.Descriptor{
		Kind: &pb.Descriptor_File{File: fdp},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	val := resp.GetValue()
	if !strings.Contains(val, "messages: 3") {
		t.Error("wrong message count")
	}
	if !strings.Contains(val, "services: 2") {
		t.Error("wrong service count")
	}
	if !strings.Contains(val, "enums: 1") {
		t.Error("wrong enum count")
	}
}

func TestNew(t *testing.T) {
	srv := New("test-token")
	if srv.GitHubToken != "test-token" {
		t.Error("token not set")
	}
	if srv.Logger == nil {
		t.Error("logger not initialized")
	}
}

func TestTransform_RenameService(t *testing.T) {
	srv := New("")
	resp, err := srv.Transform(context.Background(), &pb.WalkRequest{
		File: testFileDescriptor(),
		Commands: []*pb.TransformCommand{
			{Command: &pb.TransformCommand_Rename{
				Rename: &pb.RenameSymbol{
					OldName: "TestService",
					NewName: "RenamedService",
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	svc := resp.GetFile().GetService()[0]
	if svc.GetName() != "RenamedService" {
		t.Errorf("expected RenamedService, got %s", svc.GetName())
	}
}

func TestTransform_MergeAndRename(t *testing.T) {
	srv := New("")
	resp, err := srv.Transform(context.Background(), &pb.WalkRequest{
		File: testFileDescriptor(),
		Commands: []*pb.TransformCommand{
			{Command: &pb.TransformCommand_MergeMessages{
				MergeMessages: &pb.MergeMessages{
					SourceMessage: "Request",
					TargetMessage: "Response",
					ResultName:    "Unified",
				},
			}},
			{Command: &pb.TransformCommand_AddField{
				AddField: &pb.FieldModification{
					FieldName:     "version",
					FieldType:     int32(descriptorpb.FieldDescriptorProto_TYPE_INT32),
					TargetMessage: "Unified",
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	var unified *descriptorpb.DescriptorProto
	for _, m := range resp.GetFile().GetMessageType() {
		if m.GetName() == "Unified" {
			unified = m
		}
	}
	if unified == nil {
		t.Fatal("Unified not found")
	}
	fields := map[string]bool{}
	for _, f := range unified.GetField() {
		fields[f.GetName()] = true
	}
	if !fields["id"] || !fields["name"] || !fields["version"] {
		t.Errorf("expected id, name, version; got %v", fields)
	}
}
