package transform

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestAddFieldToMessage_NonExistentTarget(t *testing.T) {
	fdp := testFileDescriptor()
	// Adding to a specific message that doesn't exist should still succeed
	// (no messages match, so nothing is modified)
	result, err := AddFieldToMessage(fdp, "NonExistent", "uuid", descriptorpb.FieldDescriptorProto_TYPE_STRING, "", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify no uuid was added
	for _, m := range result.GetMessageType() {
		for _, f := range m.GetField() {
			if f.GetName() == "uuid" {
				t.Errorf("uuid should not have been added to %s", m.GetName())
			}
		}
	}
}

func TestAddFieldToMessage_AllScalarTypes(t *testing.T) {
	types := []descriptorpb.FieldDescriptorProto_Type{
		descriptorpb.FieldDescriptorProto_TYPE_INT32,
		descriptorpb.FieldDescriptorProto_TYPE_INT64,
		descriptorpb.FieldDescriptorProto_TYPE_UINT32,
		descriptorpb.FieldDescriptorProto_TYPE_UINT64,
		descriptorpb.FieldDescriptorProto_TYPE_BOOL,
		descriptorpb.FieldDescriptorProto_TYPE_FLOAT,
		descriptorpb.FieldDescriptorProto_TYPE_DOUBLE,
		descriptorpb.FieldDescriptorProto_TYPE_BYTES,
		descriptorpb.FieldDescriptorProto_TYPE_STRING,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED32,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED64,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED32,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED64,
		descriptorpb.FieldDescriptorProto_TYPE_SINT32,
		descriptorpb.FieldDescriptorProto_TYPE_SINT64,
	}
	for _, ft := range types {
		fdp := testFileDescriptor()
		_, err := AddFieldToMessage(fdp, "UserRequest", "test_field", ft, "", 0, false)
		if err != nil {
			t.Errorf("AddFieldToMessage with type %v failed: %v", ft, err)
		}
	}
}

func TestAddMethodToService_WithStreaming(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := AddMethodToService(fdp, "UserService", "StreamData", "UserRequest", "UserResponse", true, true)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	svc := result.GetService()[0]
	var found *descriptorpb.MethodDescriptorProto
	for _, m := range svc.GetMethod() {
		if m.GetName() == "StreamData" {
			found = m
			break
		}
	}
	if found == nil {
		t.Fatal("StreamData method not found")
	}
	if !found.GetClientStreaming() {
		t.Error("expected client streaming")
	}
	if !found.GetServerStreaming() {
		t.Error("expected server streaming")
	}
}

func TestAddMethodToService_StubCreation(t *testing.T) {
	fdp := testFileDescriptor()
	// Add method with types that don't exist — should create stubs
	result, err := AddMethodToService(fdp, "UserService", "NewMethod", "BrandNewReq", "BrandNewResp", false, false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// BrandNewReq and BrandNewResp should have been created
	names := map[string]bool{}
	for _, m := range result.GetMessageType() {
		names[m.GetName()] = true
	}
	if !names["BrandNewReq"] {
		t.Error("BrandNewReq stub not created")
	}
	if !names["BrandNewResp"] {
		t.Error("BrandNewResp stub not created")
	}
}

func TestAddMethodToService_SameInputOutput(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := AddMethodToService(fdp, "UserService", "Echo", "NewType", "NewType", false, false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Should create only one stub
	count := 0
	for _, m := range result.GetMessageType() {
		if m.GetName() == "NewType" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 NewType message, got %d", count)
	}
}

func TestMergeMessagesInFile_SourceNotFound(t *testing.T) {
	fdp := testFileDescriptor()
	_, err := MergeMessagesInFile(fdp, "NonExistent", "UserResponse", "")
	if err == nil {
		t.Error("expected error for non-existent source")
	}
}

func TestMergeMessagesInFile_TargetNotFound(t *testing.T) {
	fdp := testFileDescriptor()
	_, err := MergeMessagesInFile(fdp, "UserRequest", "NonExistent", "")
	if err == nil {
		t.Error("expected error for non-existent target")
	}
}

func TestMergeMessagesInFile_NoRename(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := MergeMessagesInFile(fdp, "UserRequest", "UserResponse", "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Target should keep its original name
	found := false
	for _, m := range result.GetMessageType() {
		if m.GetName() == "UserResponse" {
			found = true
		}
	}
	if !found {
		t.Error("UserResponse should still exist")
	}
}

func TestRenameSymbolInFile_NotFound(t *testing.T) {
	fdp := testFileDescriptor()
	_, err := RenameSymbolInFile(fdp, "NonExistent", "NewName")
	if err == nil {
		t.Error("expected error for non-existent symbol")
	}
}

func TestRenameSymbolInFile_Enum(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := RenameSymbolInFile(fdp, "Status", "State")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	found := false
	for _, e := range result.GetEnumType() {
		if e.GetName() == "State" {
			found = true
		}
		if e.GetName() == "Status" {
			t.Error("Status should have been renamed")
		}
	}
	if !found {
		t.Error("State not found")
	}
}

func TestRemoveFieldFromMessage_NotFound(t *testing.T) {
	fdp := testFileDescriptor()
	_, err := RemoveFieldFromMessage(fdp, "UserRequest", "nonexistent_field")
	if err == nil {
		t.Error("expected error for non-existent field")
	}
}

func TestRemoveFieldFromMessage_WrongMessage(t *testing.T) {
	fdp := testFileDescriptor()
	_, err := RemoveFieldFromMessage(fdp, "NonExistent", "name")
	if err == nil {
		t.Error("expected error for non-existent message")
	}
}

func TestCombineFiles_Empty(t *testing.T) {
	result, err := CombineFiles(nil, "empty.proto", "empty")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.GetName() != "empty.proto" {
		t.Error("wrong name")
	}
	if len(result.GetMessageType()) != 0 {
		t.Error("should have no messages")
	}
}

func TestCombineFiles_Deduplication(t *testing.T) {
	f1 := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("a.proto"),
		Syntax:  proto.String("proto3"),
		Package: proto.String("a"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Shared")},
			{Name: proto.String("OnlyA")},
		},
	}
	f2 := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("b.proto"),
		Syntax:  proto.String("proto3"),
		Package: proto.String("b"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Shared")}, // duplicate
			{Name: proto.String("OnlyB")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{Name: proto.String("Svc")},
			{Name: proto.String("Svc")}, // duplicate within file
		},
	}
	result, err := CombineFiles([]*descriptorpb.FileDescriptorProto{f1, f2}, "combined.proto", "combined")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Should have 3 unique messages: Shared, OnlyA, OnlyB
	if len(result.GetMessageType()) != 3 {
		t.Errorf("expected 3 messages, got %d", len(result.GetMessageType()))
	}
	// Should have 1 unique service
	if len(result.GetService()) != 1 {
		t.Errorf("expected 1 service, got %d", len(result.GetService()))
	}
}

func TestNewFileDescriptor(t *testing.T) {
	fdp := NewFileDescriptor("new.proto", "newpkg")
	if fdp.GetName() != "new.proto" {
		t.Error("wrong name")
	}
	if fdp.GetPackage() != "newpkg" {
		t.Error("wrong package")
	}
	if fdp.GetSyntax() != "proto3" {
		t.Error("wrong syntax")
	}
}

func TestNewMethod_Streaming(t *testing.T) {
	m := NewMethod("StreamRPC", ".pkg.Req", ".pkg.Resp", true, true)
	if m.GetName() != "StreamRPC" {
		t.Error("wrong name")
	}
	if !m.GetClientStreaming() {
		t.Error("expected client streaming")
	}
	if !m.GetServerStreaming() {
		t.Error("expected server streaming")
	}
}

func TestNewMessage_FieldOrdering(t *testing.T) {
	msg := NewMessage("Ordered", map[string]descriptorpb.FieldDescriptorProto_Type{
		"a": descriptorpb.FieldDescriptorProto_TYPE_STRING,
		"b": descriptorpb.FieldDescriptorProto_TYPE_INT32,
		"c": descriptorpb.FieldDescriptorProto_TYPE_BOOL,
	})
	if len(msg.GetField()) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(msg.GetField()))
	}
	// All field numbers should be unique and positive
	seen := map[int32]bool{}
	for _, f := range msg.GetField() {
		n := f.GetNumber()
		if n <= 0 {
			t.Errorf("field %s has invalid number %d", f.GetName(), n)
		}
		if seen[n] {
			t.Errorf("duplicate field number %d", n)
		}
		seen[n] = true
	}
}

func TestAddFieldToMessage_ExplicitFieldNumber(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := AddFieldToMessage(fdp, "UserRequest", "priority", descriptorpb.FieldDescriptorProto_TYPE_INT32, "", 99, false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	for _, m := range result.GetMessageType() {
		if m.GetName() == "UserRequest" {
			for _, f := range m.GetField() {
				if f.GetName() == "priority" {
					if f.GetNumber() != 99 {
						t.Errorf("expected field number 99, got %d", f.GetNumber())
					}
					return
				}
			}
			t.Error("priority field not found")
		}
	}
}
