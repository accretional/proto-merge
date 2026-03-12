package descriptor

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestDebugInterceptor_Success(t *testing.T) {
	logger := NewLogger()
	interceptor := DebugInterceptor(logger)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return &descriptorpb.FileDescriptorProto{Name: proto.String("test.proto")}, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	resp, err := interceptor(context.Background(), "request", info, handler)
	if err != nil {
		t.Fatalf("interceptor error: %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}

	if len(logger.Entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logger.Entries))
	}
	if !strings.Contains(logger.Entries[0].Operation, "rpc-start") {
		t.Errorf("first entry should be rpc-start, got %s", logger.Entries[0].Operation)
	}
	if !strings.Contains(logger.Entries[1].Operation, "rpc-done") {
		t.Errorf("second entry should be rpc-done, got %s", logger.Entries[1].Operation)
	}
}

func TestDebugInterceptor_Error(t *testing.T) {
	logger := NewLogger()
	interceptor := DebugInterceptor(logger)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, fmt.Errorf("test error")
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Fail"}
	_, err := interceptor(context.Background(), "request", info, handler)
	if err == nil {
		t.Fatal("expected error")
	}

	if len(logger.Entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logger.Entries))
	}
	if !strings.Contains(logger.Entries[1].Detail, "test error") {
		t.Errorf("error not logged: %s", logger.Entries[1].Detail)
	}
}

func TestDebugInterceptor_NilRequest(t *testing.T) {
	logger := NewLogger()
	interceptor := DebugInterceptor(logger)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Nil"}
	_, _ = interceptor(context.Background(), nil, info, handler)

	if len(logger.Entries) < 2 {
		t.Fatal("expected at least 2 log entries")
	}
	if !strings.Contains(logger.Entries[0].Detail, "<nil>") {
		t.Errorf("nil request not logged: %s", logger.Entries[0].Detail)
	}
}

type mockServerStream struct {
	grpc.ServerStream
}

func TestStreamDebugInterceptor_Success(t *testing.T) {
	logger := NewLogger()
	interceptor := StreamDebugInterceptor(logger)

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return nil
	}

	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/Stream",
		IsClientStream: true,
		IsServerStream: true,
	}

	err := interceptor(nil, &mockServerStream{}, info, handler)
	if err != nil {
		t.Fatalf("interceptor error: %v", err)
	}

	if len(logger.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(logger.Entries))
	}
	if !strings.Contains(logger.Entries[0].Detail, "client_stream=true") {
		t.Error("client_stream not logged")
	}
	if !strings.Contains(logger.Entries[1].Operation, "stream-done") {
		t.Error("expected stream-done")
	}
}

func TestStreamDebugInterceptor_Error(t *testing.T) {
	logger := NewLogger()
	interceptor := StreamDebugInterceptor(logger)

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return fmt.Errorf("stream failure")
	}

	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamFail"}
	err := interceptor(nil, &mockServerStream{}, info, handler)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(logger.Entries[1].Detail, "stream failure") {
		t.Errorf("error not logged: %s", logger.Entries[1].Detail)
	}
}

func TestDescribeMessage_WithNestedAndOneof(t *testing.T) {
	msg := &descriptorpb.DescriptorProto{
		Name: proto.String("Complex"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:   proto.String("id"),
				Number: proto.Int32(1),
				Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
				Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			},
			{
				Name:   proto.String("tags"),
				Number: proto.Int32(2),
				Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
			},
		},
		NestedType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Nested"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("inner"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
		OneofDecl: []*descriptorpb.OneofDescriptorProto{
			{Name: proto.String("payload")},
		},
	}

	result := DescribeMessage(msg)
	if !strings.Contains(result, "Complex") {
		t.Error("missing message name")
	}
	if !strings.Contains(result, "repeated") {
		t.Error("missing repeated label")
	}
	if !strings.Contains(result, "Nested") {
		t.Error("missing nested message")
	}
	if !strings.Contains(result, "oneof payload") {
		t.Error("missing oneof")
	}
}

func TestDescribeService_Streaming(t *testing.T) {
	svc := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String("StreamSvc"),
		Method: []*descriptorpb.MethodDescriptorProto{
			{
				Name:            proto.String("BidiStream"),
				InputType:       proto.String(".pkg.Req"),
				OutputType:      proto.String(".pkg.Resp"),
				ClientStreaming: proto.Bool(true),
				ServerStreaming: proto.Bool(true),
			},
			{
				Name:            proto.String("ServerStream"),
				InputType:       proto.String(".pkg.Req"),
				OutputType:      proto.String(".pkg.Resp"),
				ServerStreaming: proto.Bool(true),
			},
		},
	}

	result := DescribeService(svc)
	if !strings.Contains(result, "stream .pkg.Req") {
		t.Error("missing client stream keyword")
	}
	// Count "stream " occurrences — bidi has 2, server-only has 1 = 3 total
	count := strings.Count(result, "stream ")
	if count != 3 {
		t.Errorf("expected 3 'stream ' occurrences, got %d in:\n%s", count, result)
	}
}

func TestDescribeMethod_Streaming(t *testing.T) {
	m := &descriptorpb.MethodDescriptorProto{
		Name:            proto.String("BiDi"),
		InputType:       proto.String(".x.In"),
		OutputType:      proto.String(".x.Out"),
		ClientStreaming: proto.Bool(true),
		ServerStreaming: proto.Bool(true),
	}
	result := DescribeMethod(m)
	if !strings.Contains(result, "stream .x.In") {
		t.Error("missing client stream")
	}
	if !strings.Contains(result, "stream .x.Out") {
		t.Error("missing server stream")
	}
}

func TestDescribeField_AllTypes(t *testing.T) {
	types := []struct {
		typ  descriptorpb.FieldDescriptorProto_Type
		want string
	}{
		{descriptorpb.FieldDescriptorProto_TYPE_INT32, "int32"},
		{descriptorpb.FieldDescriptorProto_TYPE_INT64, "int64"},
		{descriptorpb.FieldDescriptorProto_TYPE_UINT32, "uint32"},
		{descriptorpb.FieldDescriptorProto_TYPE_UINT64, "uint64"},
		{descriptorpb.FieldDescriptorProto_TYPE_BOOL, "bool"},
		{descriptorpb.FieldDescriptorProto_TYPE_FLOAT, "float"},
		{descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, "double"},
		{descriptorpb.FieldDescriptorProto_TYPE_BYTES, "bytes"},
		{descriptorpb.FieldDescriptorProto_TYPE_STRING, "string"},
	}
	for _, tt := range types {
		f := &descriptorpb.FieldDescriptorProto{
			Name:   proto.String("x"),
			Number: proto.Int32(1),
			Type:   tt.typ.Enum(),
			Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		}
		result := DescribeField(f)
		if !strings.Contains(result, tt.want) {
			t.Errorf("type %v: expected %q in %q", tt.typ, tt.want, result)
		}
	}
}

func TestDescribeField_MessageType(t *testing.T) {
	f := &descriptorpb.FieldDescriptorProto{
		Name:     proto.String("sub"),
		Number:   proto.Int32(1),
		Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
		TypeName: proto.String(".pkg.SubMsg"),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
	}
	result := DescribeField(f)
	if !strings.Contains(result, ".pkg.SubMsg") {
		t.Errorf("expected type name in: %s", result)
	}
}

func TestLoggerStrings_Empty(t *testing.T) {
	l := NewLogger()
	s := l.Strings()
	if len(s) != 0 {
		t.Errorf("expected empty, got %d", len(s))
	}
}

func TestLoggerMultipleEntries(t *testing.T) {
	l := NewLogger()
	l.Log("a", "b", "c")
	l.Log("d", "e", "f")
	l.Log("g", "h", "i")
	if len(l.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(l.Entries))
	}
	s := l.Strings()
	if len(s) != 3 {
		t.Errorf("expected 3 strings, got %d", len(s))
	}
}

func TestToString_Invalid(t *testing.T) {
	// A descriptor with a type reference that can't be resolved
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("bad.proto"),
		Package: proto.String("bad"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Msg"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("ref"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".nonexistent.Type"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}
	_, err := ToString(fdp)
	if err == nil {
		t.Error("expected error for unresolved type reference")
	}
}
