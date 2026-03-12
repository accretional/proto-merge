package descriptor

import (
	"strings"
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
				Name: proto.String("HelloRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("name"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
					{
						Name:   proto.String("count"),
						Number: proto.Int32(2),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
			{
				Name: proto.String("HelloResponse"),
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
				Name: proto.String("Greeter"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("SayHello"),
						InputType:  proto.String(".test.HelloRequest"),
						OutputType: proto.String(".test.HelloResponse"),
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

func TestDescribeMessage(t *testing.T) {
	fdp := testFileDescriptor()
	result := DescribeMessage(fdp.GetMessageType()[0])
	if !strings.Contains(result, "HelloRequest") {
		t.Error("missing message name")
	}
	if !strings.Contains(result, "name") {
		t.Error("missing field name")
	}
	if !strings.Contains(result, "string") {
		t.Error("missing field type")
	}
}

func TestDescribeService(t *testing.T) {
	fdp := testFileDescriptor()
	result := DescribeService(fdp.GetService()[0])
	if !strings.Contains(result, "Greeter") {
		t.Error("missing service name")
	}
	if !strings.Contains(result, "SayHello") {
		t.Error("missing method name")
	}
}

func TestDescribeField(t *testing.T) {
	fdp := testFileDescriptor()
	result := DescribeField(fdp.GetMessageType()[0].GetField()[0])
	if !strings.Contains(result, "name") || !strings.Contains(result, "string") {
		t.Errorf("unexpected field description: %s", result)
	}
}

func TestDescribeEnum(t *testing.T) {
	fdp := testFileDescriptor()
	result := DescribeEnum(fdp.GetEnumType()[0])
	if !strings.Contains(result, "Status") {
		t.Error("missing enum name")
	}
	if !strings.Contains(result, "ACTIVE") {
		t.Error("missing enum value")
	}
}

func TestDescribeFile(t *testing.T) {
	fdp := testFileDescriptor()
	result := DescribeFile(fdp)
	if !strings.Contains(result, "test.proto") {
		t.Error("missing file name")
	}
	if !strings.Contains(result, "messages: 2") {
		t.Error("wrong message count")
	}
	if !strings.Contains(result, "services: 1") {
		t.Error("wrong service count")
	}
}

func TestToString(t *testing.T) {
	fdp := testFileDescriptor()
	result, err := ToString(fdp)
	if err != nil {
		t.Fatalf("ToString failed: %v", err)
	}
	if !strings.Contains(result, "syntax") {
		t.Error("missing syntax in output")
	}
	if !strings.Contains(result, "HelloRequest") {
		t.Error("missing message in output")
	}
	if !strings.Contains(result, "Greeter") {
		t.Error("missing service in output")
	}
}

func TestLogger(t *testing.T) {
	l := NewLogger()
	l.Log("test-op", "target", "detail")
	if len(l.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(l.Entries))
	}
	s := l.Strings()
	if !strings.Contains(s[0], "test-op") {
		t.Error("log entry missing operation")
	}
}

func TestLogFileDescriptor(t *testing.T) {
	l := NewLogger()
	fdp := testFileDescriptor()
	l.LogFileDescriptor(fdp)
	if len(l.Entries) != 1 {
		t.Fatal("expected 1 entry")
	}
	if !strings.Contains(l.Entries[0].Detail, "2 messages") {
		t.Error("wrong message count in log")
	}
}
