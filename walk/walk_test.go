package walk

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
	}
}

func TestWalk_NoCommands(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := Walk(fdp, nil)
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}
	if result.File.GetName() != "test.proto" {
		t.Error("file name mismatch")
	}
	if len(result.Log) == 0 {
		t.Error("expected log entries from walk")
	}

	// Verify walk visited messages
	hasVisitMsg := false
	for _, entry := range result.Log {
		if len(entry) > 0 {
			hasVisitMsg = true
		}
	}
	if !hasVisitMsg {
		t.Error("expected visit entries in log")
	}
}

func TestWalk_AddUUIDToAllMessages(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := Walk(fdp, []TransformCommand{
		{
			AddField: &FieldModification{
				FieldName:     "uuid",
				FieldType:     descriptorpb.FieldDescriptorProto_TYPE_STRING,
				TargetMessage: "", // all messages
			},
		},
	})
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	for _, msg := range result.File.GetMessageType() {
		found := false
		for _, f := range msg.GetField() {
			if f.GetName() == "uuid" {
				found = true
			}
		}
		if !found {
			t.Errorf("uuid not found in %s", msg.GetName())
		}
	}
}

func TestWalk_AddIdentityAPIs(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := Walk(fdp, []TransformCommand{
		{
			AddMethod: &ServiceModification{
				MethodName:    "Identify",
				InputType:     "UserRequest",
				OutputType:    "UserResponse",
				TargetService: "", // all services
			},
		},
		{
			AddMethod: &ServiceModification{
				MethodName:    "Discover",
				InputType:     "UserRequest",
				OutputType:    "UserResponse",
				TargetService: "", // all services
			},
		},
	})
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	svc := result.File.GetService()[0]
	methodNames := map[string]bool{}
	for _, m := range svc.GetMethod() {
		methodNames[m.GetName()] = true
	}
	if !methodNames["GetUser"] {
		t.Error("missing original GetUser method")
	}
	if !methodNames["Identify"] {
		t.Error("missing Identify method")
	}
	if !methodNames["Discover"] {
		t.Error("missing Discover method")
	}
}

func TestWalk_ChainedTransforms(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := Walk(fdp, []TransformCommand{
		// Add uuid to all messages
		{
			AddField: &FieldModification{
				FieldName:     "uuid",
				FieldType:     descriptorpb.FieldDescriptorProto_TYPE_STRING,
				TargetMessage: "",
			},
		},
		// Rename UserRequest to AccountRequest
		{
			Rename: &RenameSymbol{
				OldName: "UserRequest",
				NewName: "AccountRequest",
			},
		},
		// Add Ping to all services
		{
			AddMethod: &ServiceModification{
				MethodName:    "Ping",
				InputType:     "AccountRequest",
				OutputType:    "UserResponse",
				TargetService: "",
			},
		},
	})
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	// Verify rename happened
	msgNames := map[string]bool{}
	for _, m := range result.File.GetMessageType() {
		msgNames[m.GetName()] = true
	}
	if msgNames["UserRequest"] {
		t.Error("UserRequest should have been renamed")
	}
	if !msgNames["AccountRequest"] {
		t.Error("AccountRequest should exist")
	}

	// Verify all messages have uuid
	for _, m := range result.File.GetMessageType() {
		found := false
		for _, f := range m.GetField() {
			if f.GetName() == "uuid" {
				found = true
			}
		}
		if !found {
			t.Errorf("uuid missing from %s", m.GetName())
		}
	}

	// Verify Ping added
	svc := result.File.GetService()[0]
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

func TestWalk_MergeAndRemove(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := Walk(fdp, []TransformCommand{
		// Merge UserRequest into UserResponse
		{
			MergeMessages: &MergeMessages{
				SourceMessage: "UserRequest",
				TargetMessage: "UserResponse",
				ResultName:    "CombinedUser",
			},
		},
	})
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	var combined *descriptorpb.DescriptorProto
	for _, m := range result.File.GetMessageType() {
		if m.GetName() == "CombinedUser" {
			combined = m
		}
	}
	if combined == nil {
		t.Fatal("CombinedUser not found")
	}

	fields := map[string]bool{}
	for _, f := range combined.GetField() {
		fields[f.GetName()] = true
	}
	if !fields["id"] {
		t.Error("missing id field")
	}
	if !fields["name"] {
		t.Error("missing name field")
	}
}

func TestWalkMessages(t *testing.T) {
	fdp := testFileDescriptor()
	var visited []string
	err := WalkMessages(fdp, func(path string, msg *descriptorpb.DescriptorProto) error {
		visited = append(visited, path)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkMessages failed: %v", err)
	}
	if len(visited) != 2 {
		t.Errorf("expected 2 messages visited, got %d: %v", len(visited), visited)
	}
}

func TestWalkServices(t *testing.T) {
	fdp := testFileDescriptor()
	var visited []string
	err := WalkServices(fdp, func(svc *descriptorpb.ServiceDescriptorProto) error {
		visited = append(visited, svc.GetName())
		return nil
	})
	if err != nil {
		t.Fatalf("WalkServices failed: %v", err)
	}
	if len(visited) != 1 || visited[0] != "UserService" {
		t.Errorf("expected [UserService], got %v", visited)
	}
}

func TestWalk_NilFile(t *testing.T) {
	_, err := Walk(nil, nil)
	if err == nil {
		t.Error("expected error for nil file")
	}
}
