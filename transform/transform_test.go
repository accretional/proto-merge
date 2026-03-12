package transform

import (
	"testing"

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
				Name: proto.String("UserRequest"),
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
				Name: proto.String("UserResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("greeting"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("UserService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("GetUser"),
						InputType:  proto.String(".test.UserRequest"),
						OutputType: proto.String(".test.UserResponse"),
					},
				},
			},
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("Status"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("UNKNOWN"), Number: proto.Int32(0)},
					{Name: proto.String("ACTIVE"), Number: proto.Int32(1)},
				},
			},
		},
	}
}

func TestAddFieldToMessage_Specific(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := AddFieldToMessage(fdp, "UserRequest", "uuid", descriptorpb.FieldDescriptorProto_TYPE_STRING, "", 10, false)
	if err != nil {
		t.Fatalf("AddFieldToMessage failed: %v", err)
	}

	found := false
	for _, m := range result.GetMessageType() {
		if m.GetName() == "UserRequest" {
			for _, f := range m.GetField() {
				if f.GetName() == "uuid" {
					found = true
					if f.GetType() != descriptorpb.FieldDescriptorProto_TYPE_STRING {
						t.Error("uuid field should be string type")
					}
				}
			}
		}
	}
	if !found {
		t.Error("uuid field not found in UserRequest")
	}

	// Verify UserResponse was NOT modified
	for _, m := range result.GetMessageType() {
		if m.GetName() == "UserResponse" {
			for _, f := range m.GetField() {
				if f.GetName() == "uuid" {
					t.Error("uuid field should NOT be in UserResponse")
				}
			}
		}
	}
}

func TestAddFieldToMessage_AllMessages(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := AddFieldToMessage(fdp, "", "uuid", descriptorpb.FieldDescriptorProto_TYPE_STRING, "", 0, false)
	if err != nil {
		t.Fatalf("AddFieldToMessage (all) failed: %v", err)
	}

	for _, m := range result.GetMessageType() {
		found := false
		for _, f := range m.GetField() {
			if f.GetName() == "uuid" {
				found = true
			}
		}
		if !found {
			t.Errorf("uuid field not found in %s", m.GetName())
		}
	}
}

func TestAddFieldRepeated(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := AddFieldToMessage(fdp, "UserRequest", "tags", descriptorpb.FieldDescriptorProto_TYPE_STRING, "", 0, true)
	if err != nil {
		t.Fatalf("AddFieldToMessage (repeated) failed: %v", err)
	}

	for _, m := range result.GetMessageType() {
		if m.GetName() == "UserRequest" {
			for _, f := range m.GetField() {
				if f.GetName() == "tags" {
					if f.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
						t.Error("tags field should be repeated")
					}
					return
				}
			}
			t.Error("tags field not found")
		}
	}
}

func TestAddMethodToService(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := AddMethodToService(fdp, "UserService", "DeleteUser", "UserRequest", "UserResponse", false, false)
	if err != nil {
		t.Fatalf("AddMethodToService failed: %v", err)
	}

	svc := result.GetService()[0]
	if len(svc.GetMethod()) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(svc.GetMethod()))
	}

	found := false
	for _, m := range svc.GetMethod() {
		if m.GetName() == "DeleteUser" {
			found = true
		}
	}
	if !found {
		t.Error("DeleteUser method not found")
	}
}

func TestAddMethodToService_AllServices(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := AddMethodToService(fdp, "", "Ping", "UserRequest", "UserResponse", false, false)
	if err != nil {
		t.Fatalf("AddMethodToService (all) failed: %v", err)
	}

	for _, svc := range result.GetService() {
		found := false
		for _, m := range svc.GetMethod() {
			if m.GetName() == "Ping" {
				found = true
			}
		}
		if !found {
			t.Errorf("Ping method not found in %s", svc.GetName())
		}
	}
}

func TestMergeMessagesInFile(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := MergeMessagesInFile(fdp, "UserRequest", "UserResponse", "CombinedUser")
	if err != nil {
		t.Fatalf("MergeMessagesInFile failed: %v", err)
	}

	var combined *descriptorpb.DescriptorProto
	for _, m := range result.GetMessageType() {
		if m.GetName() == "CombinedUser" {
			combined = m
			break
		}
	}
	if combined == nil {
		t.Fatal("CombinedUser message not found")
	}

	fieldNames := map[string]bool{}
	for _, f := range combined.GetField() {
		fieldNames[f.GetName()] = true
	}
	// Should have greeting (original) + name (merged from UserRequest)
	if !fieldNames["greeting"] {
		t.Error("missing greeting field")
	}
	if !fieldNames["name"] {
		t.Error("missing name field (merged from UserRequest)")
	}
}

func TestRenameSymbolInFile(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := RenameSymbolInFile(fdp, "UserRequest", "AccountRequest")
	if err != nil {
		t.Fatalf("RenameSymbolInFile failed: %v", err)
	}

	found := false
	for _, m := range result.GetMessageType() {
		if m.GetName() == "AccountRequest" {
			found = true
		}
		if m.GetName() == "UserRequest" {
			t.Error("UserRequest should have been renamed")
		}
	}
	if !found {
		t.Error("AccountRequest not found")
	}
}

func TestRenameSymbolInFile_Service(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := RenameSymbolInFile(fdp, "UserService", "AccountService")
	if err != nil {
		t.Fatalf("RenameSymbolInFile (service) failed: %v", err)
	}

	if result.GetService()[0].GetName() != "AccountService" {
		t.Errorf("expected AccountService, got %s", result.GetService()[0].GetName())
	}
}

func TestRemoveFieldFromMessage(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := RemoveFieldFromMessage(fdp, "UserRequest", "name")
	if err != nil {
		t.Fatalf("RemoveFieldFromMessage failed: %v", err)
	}

	for _, m := range result.GetMessageType() {
		if m.GetName() == "UserRequest" {
			for _, f := range m.GetField() {
				if f.GetName() == "name" {
					t.Error("name field should have been removed")
				}
			}
			return
		}
	}
	t.Error("UserRequest not found in result")
}

func TestCombineFiles(t *testing.T) {
	fdp1 := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("a.proto"),
		Package: proto.String("a"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Foo")},
		},
	}
	fdp2 := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("b.proto"),
		Package: proto.String("b"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Bar")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{Name: proto.String("Baz")},
		},
	}

	result, err := CombineFiles([]*descriptorpb.FileDescriptorProto{fdp1, fdp2}, "combined.proto", "combined")
	if err != nil {
		t.Fatalf("CombineFiles failed: %v", err)
	}

	if len(result.GetMessageType()) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result.GetMessageType()))
	}
	if len(result.GetService()) != 1 {
		t.Errorf("expected 1 service, got %d", len(result.GetService()))
	}
}

func TestNewMessage(t *testing.T) {
	msg := NewMessage("TestMsg", map[string]descriptorpb.FieldDescriptorProto_Type{
		"id":   descriptorpb.FieldDescriptorProto_TYPE_INT64,
		"name": descriptorpb.FieldDescriptorProto_TYPE_STRING,
	})
	if msg.GetName() != "TestMsg" {
		t.Error("wrong name")
	}
	if len(msg.GetField()) != 2 {
		t.Errorf("expected 2 fields, got %d", len(msg.GetField()))
	}
}

func TestNewService(t *testing.T) {
	svc := NewService("TestSvc", []*descriptorpb.MethodDescriptorProto{
		NewMethod("Do", ".pkg.Req", ".pkg.Resp", false, false),
	})
	if svc.GetName() != "TestSvc" {
		t.Error("wrong name")
	}
	if len(svc.GetMethod()) != 1 {
		t.Error("expected 1 method")
	}
}
