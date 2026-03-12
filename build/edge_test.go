package build

import (
	"testing"

	"github.com/accretional/merge/walk"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestRunLocal_EmptyFiles(t *testing.T) {
	result, err := RunLocal(nil, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Bundled == nil {
		t.Fatal("bundled should not be nil")
	}
	if len(result.Bundled.GetMessageType()) != 0 {
		t.Error("should have no messages")
	}
}

func TestRunLocal_TransformError(t *testing.T) {
	files := []*descriptorpb.FileDescriptorProto{
		{
			Name:    proto.String("test.proto"),
			Package: proto.String("test"),
			Syntax:  proto.String("proto3"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("Msg"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{
							Name:   proto.String("x"),
							Number: proto.Int32(1),
							Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						},
					},
				},
			},
		},
	}
	// Invalid merge should not crash — file should be kept as-is
	result, err := RunLocal(files, []walk.TransformCommand{
		{
			MergeMessages: &walk.MergeMessages{
				SourceMessage: "NonExistent",
				TargetMessage: "AlsoNonExistent",
			},
		},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Original file should be preserved despite transform error
	if len(result.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(result.Files))
	}
}

func TestRunLocal_MultipleTransforms(t *testing.T) {
	files := testFiles()
	result, err := RunLocal(files, []walk.TransformCommand{
		// Add uuid
		{AddField: &walk.FieldModification{
			FieldName: "uuid",
			FieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING,
		}},
		// Add created_at
		{AddField: &walk.FieldModification{
			FieldName: "created_at",
			FieldType: descriptorpb.FieldDescriptorProto_TYPE_INT64,
		}},
		// Add updated_at
		{AddField: &walk.FieldModification{
			FieldName: "updated_at",
			FieldType: descriptorpb.FieldDescriptorProto_TYPE_INT64,
		}},
		// Add Ping to all services
		{AddMethod: &walk.ServiceModification{
			MethodName: "Ping",
			InputType:  "UserRequest",
			OutputType: "UserResponse",
		}},
		// Add Version to all services
		{AddMethod: &walk.ServiceModification{
			MethodName: "Version",
			InputType:  "UserRequest",
			OutputType: "UserResponse",
		}},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Check that messages from users.proto have all 3 new fields
	// (orders.proto gets stub UserRequest/UserResponse from AddMethod which won't have them)
	for _, m := range result.Files[0].GetMessageType() {
		fields := map[string]bool{}
		for _, fld := range m.GetField() {
			fields[fld.GetName()] = true
		}
		for _, expected := range []string{"uuid", "created_at", "updated_at"} {
			if !fields[expected] {
				t.Errorf("%s missing from %s in %s", expected, m.GetName(), result.Files[0].GetName())
			}
		}
	}
	// Check OrderRequest in orders.proto
	for _, m := range result.Files[1].GetMessageType() {
		if m.GetName() != "OrderRequest" {
			continue
		}
		fields := map[string]bool{}
		for _, fld := range m.GetField() {
			fields[fld.GetName()] = true
		}
		for _, expected := range []string{"uuid", "created_at", "updated_at"} {
			if !fields[expected] {
				t.Errorf("%s missing from OrderRequest in orders.proto", expected)
			}
		}
	}

	// Check all services have both new methods
	for _, f := range result.Files {
		for _, svc := range f.GetService() {
			methods := map[string]bool{}
			for _, m := range svc.GetMethod() {
				methods[m.GetName()] = true
			}
			for _, expected := range []string{"Ping", "Version"} {
				if !methods[expected] {
					t.Errorf("%s missing from %s in %s", expected, svc.GetName(), f.GetName())
				}
			}
		}
	}
}

func TestRunLocal_BundleDeduplication(t *testing.T) {
	files := []*descriptorpb.FileDescriptorProto{
		{
			Name:    proto.String("a.proto"),
			Package: proto.String("a"),
			Syntax:  proto.String("proto3"),
			MessageType: []*descriptorpb.DescriptorProto{
				{Name: proto.String("Shared")},
				{Name: proto.String("OnlyA")},
			},
		},
		{
			Name:    proto.String("b.proto"),
			Package: proto.String("b"),
			Syntax:  proto.String("proto3"),
			MessageType: []*descriptorpb.DescriptorProto{
				{Name: proto.String("Shared")}, // duplicate
				{Name: proto.String("OnlyB")},
			},
		},
	}

	result, err := RunLocal(files, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Bundle should deduplicate
	if len(result.Bundled.GetMessageType()) != 3 {
		t.Errorf("expected 3 messages in bundle, got %d", len(result.Bundled.GetMessageType()))
	}
}

func TestRunLocal_LogOutput(t *testing.T) {
	files := testFiles()
	result, err := RunLocal(files, []walk.TransformCommand{
		{AddField: &walk.FieldModification{
			FieldName: "test_field",
			FieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING,
		}},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Log) == 0 {
		t.Error("expected log entries")
	}
}

func TestRunLocal_PreservesOriginalFiles(t *testing.T) {
	files := testFiles()
	originalCount := len(files[0].GetMessageType()[0].GetField())

	_, err := RunLocal(files, []walk.TransformCommand{
		{AddField: &walk.FieldModification{
			FieldName: "injected",
			FieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING,
		}},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Walk clones the input, so original should be untouched
	if len(files[0].GetMessageType()[0].GetField()) != originalCount {
		t.Error("original file was mutated")
	}
}
