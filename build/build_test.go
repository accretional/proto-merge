package build

import (
	"testing"

	"github.com/accretional/merge/walk"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func testFiles() []*descriptorpb.FileDescriptorProto {
	return []*descriptorpb.FileDescriptorProto{
		{
			Name:    proto.String("users.proto"),
			Package: proto.String("users"),
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
							InputType:  proto.String(".users.UserRequest"),
							OutputType: proto.String(".users.UserResponse"),
						},
					},
				},
			},
		},
		{
			Name:    proto.String("orders.proto"),
			Package: proto.String("orders"),
			Syntax:  proto.String("proto3"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("OrderRequest"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{
							Name:   proto.String("order_id"),
							Number: proto.Int32(1),
							Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						},
					},
				},
			},
			Service: []*descriptorpb.ServiceDescriptorProto{
				{
					Name: proto.String("OrderService"),
					Method: []*descriptorpb.MethodDescriptorProto{
						{
							Name:       proto.String("GetOrder"),
							InputType:  proto.String(".orders.OrderRequest"),
							OutputType: proto.String(".orders.OrderRequest"),
						},
					},
				},
			},
		},
	}
}

func TestRunLocal_NoCommands(t *testing.T) {
	files := testFiles()
	result, err := RunLocal(files, nil)
	if err != nil {
		t.Fatalf("RunLocal failed: %v", err)
	}
	if len(result.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(result.Files))
	}
	if result.Bundled == nil {
		t.Fatal("bundled is nil")
	}
	// Bundle should have messages from both files
	if len(result.Bundled.GetMessageType()) != 3 {
		t.Errorf("expected 3 bundled messages, got %d", len(result.Bundled.GetMessageType()))
	}
	if len(result.Bundled.GetService()) != 2 {
		t.Errorf("expected 2 bundled services, got %d", len(result.Bundled.GetService()))
	}
}

func TestRunLocal_AddUUIDToAllMessages(t *testing.T) {
	files := testFiles()
	result, err := RunLocal(files, []walk.TransformCommand{
		{
			AddField: &walk.FieldModification{
				FieldName:     "uuid",
				FieldType:     descriptorpb.FieldDescriptorProto_TYPE_STRING,
				TargetMessage: "", // all messages
			},
		},
	})
	if err != nil {
		t.Fatalf("RunLocal failed: %v", err)
	}

	// Every message in every file should have uuid
	for _, f := range result.Files {
		for _, m := range f.GetMessageType() {
			found := false
			for _, field := range m.GetField() {
				if field.GetName() == "uuid" {
					found = true
				}
			}
			if !found {
				t.Errorf("uuid missing from %s in %s", m.GetName(), f.GetName())
			}
		}
	}

	// Bundle should also have uuid on all messages
	for _, m := range result.Bundled.GetMessageType() {
		found := false
		for _, field := range m.GetField() {
			if field.GetName() == "uuid" {
				found = true
			}
		}
		if !found {
			t.Errorf("uuid missing from bundled %s", m.GetName())
		}
	}
}

func TestRunLocal_AddIdentityAPIs(t *testing.T) {
	files := testFiles()
	result, err := RunLocal(files, []walk.TransformCommand{
		{
			AddMethod: &walk.ServiceModification{
				MethodName:    "Identify",
				InputType:     "UserRequest",
				OutputType:    "UserResponse",
				TargetService: "", // all services
			},
		},
		{
			AddMethod: &walk.ServiceModification{
				MethodName:    "Discover",
				InputType:     "UserRequest",
				OutputType:    "UserResponse",
				TargetService: "", // all services
			},
		},
	})
	if err != nil {
		t.Fatalf("RunLocal failed: %v", err)
	}

	// Both services should have Identify and Discover
	for _, f := range result.Files {
		for _, svc := range f.GetService() {
			methods := map[string]bool{}
			for _, m := range svc.GetMethod() {
				methods[m.GetName()] = true
			}
			if !methods["Identify"] {
				t.Errorf("Identify missing from %s in %s", svc.GetName(), f.GetName())
			}
			if !methods["Discover"] {
				t.Errorf("Discover missing from %s in %s", svc.GetName(), f.GetName())
			}
		}
	}
}

func TestRunLocal_CombinedPipeline(t *testing.T) {
	files := testFiles()
	result, err := RunLocal(files, []walk.TransformCommand{
		// Add uuid to all messages
		{
			AddField: &walk.FieldModification{
				FieldName: "uuid",
				FieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING,
			},
		},
		// Add created_at to all messages
		{
			AddField: &walk.FieldModification{
				FieldName: "created_at",
				FieldType: descriptorpb.FieldDescriptorProto_TYPE_INT64,
			},
		},
		// Add identity APIs to all services
		{
			AddMethod: &walk.ServiceModification{
				MethodName: "Ping",
				InputType:  "UserRequest",
				OutputType: "UserResponse",
			},
		},
	})
	if err != nil {
		t.Fatalf("RunLocal failed: %v", err)
	}

	// Verify transformations propagated to bundle
	for _, m := range result.Bundled.GetMessageType() {
		fields := map[string]bool{}
		for _, f := range m.GetField() {
			fields[f.GetName()] = true
		}
		if !fields["uuid"] {
			t.Errorf("uuid missing from bundled %s", m.GetName())
		}
		if !fields["created_at"] {
			t.Errorf("created_at missing from bundled %s", m.GetName())
		}
	}

	if len(result.Log) == 0 {
		t.Error("expected log entries")
	}
}
