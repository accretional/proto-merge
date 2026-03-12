// Package descriptor provides utilities for converting protobuf descriptors
// into human-readable string output, debug logging, and gRPC interceptors.
package descriptor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoprint"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ToString converts a FileDescriptorProto to formatted proto source text.
func ToString(fdp *descriptorpb.FileDescriptorProto) (string, error) {
	fd, err := desc.CreateFileDescriptor(fdp)
	if err != nil {
		return "", fmt.Errorf("creating file descriptor: %w", err)
	}
	printer := &protoprint.Printer{
		Compact: false,
	}
	return printer.PrintProtoToString(fd)
}

// DescribeMessage returns a human-readable summary of a DescriptorProto.
func DescribeMessage(md *descriptorpb.DescriptorProto) string {
	var b strings.Builder
	fmt.Fprintf(&b, "message %s {\n", md.GetName())
	for _, f := range md.GetField() {
		label := labelStr(f.GetLabel())
		typeName := typeStr(f)
		fmt.Fprintf(&b, "  %s%s %s = %d;\n", label, typeName, f.GetName(), f.GetNumber())
	}
	for _, nested := range md.GetNestedType() {
		for _, line := range strings.Split(DescribeMessage(nested), "\n") {
			if line != "" {
				fmt.Fprintf(&b, "  %s\n", line)
			}
		}
	}
	for _, e := range md.GetEnumType() {
		fmt.Fprintf(&b, "  %s\n", DescribeEnum(e))
	}
	for _, oo := range md.GetOneofDecl() {
		fmt.Fprintf(&b, "  oneof %s { ... }\n", oo.GetName())
	}
	b.WriteString("}")
	return b.String()
}

// DescribeService returns a human-readable summary of a ServiceDescriptorProto.
func DescribeService(sd *descriptorpb.ServiceDescriptorProto) string {
	var b strings.Builder
	fmt.Fprintf(&b, "service %s {\n", sd.GetName())
	for _, m := range sd.GetMethod() {
		cs := ""
		if m.GetClientStreaming() {
			cs = "stream "
		}
		ss := ""
		if m.GetServerStreaming() {
			ss = "stream "
		}
		fmt.Fprintf(&b, "  rpc %s(%s%s) returns (%s%s);\n",
			m.GetName(), cs, m.GetInputType(), ss, m.GetOutputType())
	}
	b.WriteString("}")
	return b.String()
}

// DescribeField returns a human-readable summary of a FieldDescriptorProto.
func DescribeField(f *descriptorpb.FieldDescriptorProto) string {
	label := labelStr(f.GetLabel())
	typeName := typeStr(f)
	return fmt.Sprintf("%s%s %s = %d", label, typeName, f.GetName(), f.GetNumber())
}

// DescribeEnum returns a human-readable summary of an EnumDescriptorProto.
func DescribeEnum(e *descriptorpb.EnumDescriptorProto) string {
	var b strings.Builder
	fmt.Fprintf(&b, "enum %s { ", e.GetName())
	for i, v := range e.GetValue() {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s=%d", v.GetName(), v.GetNumber())
	}
	b.WriteString(" }")
	return b.String()
}

// DescribeMethod returns a human-readable summary of a MethodDescriptorProto.
func DescribeMethod(m *descriptorpb.MethodDescriptorProto) string {
	cs := ""
	if m.GetClientStreaming() {
		cs = "stream "
	}
	ss := ""
	if m.GetServerStreaming() {
		ss = "stream "
	}
	return fmt.Sprintf("rpc %s(%s%s) returns (%s%s)",
		m.GetName(), cs, m.GetInputType(), ss, m.GetOutputType())
}

// DescribeFile returns a human-readable summary of a FileDescriptorProto.
func DescribeFile(fdp *descriptorpb.FileDescriptorProto) string {
	var b strings.Builder
	fmt.Fprintf(&b, "file: %s\n", fdp.GetName())
	fmt.Fprintf(&b, "package: %s\n", fdp.GetPackage())
	fmt.Fprintf(&b, "syntax: %s\n", fdp.GetSyntax())
	if len(fdp.GetDependency()) > 0 {
		fmt.Fprintf(&b, "imports: %s\n", strings.Join(fdp.GetDependency(), ", "))
	}
	fmt.Fprintf(&b, "messages: %d\n", len(fdp.GetMessageType()))
	for _, m := range fdp.GetMessageType() {
		fmt.Fprintf(&b, "  %s (%d fields)\n", m.GetName(), len(m.GetField()))
	}
	fmt.Fprintf(&b, "services: %d\n", len(fdp.GetService()))
	for _, s := range fdp.GetService() {
		fmt.Fprintf(&b, "  %s (%d methods)\n", s.GetName(), len(s.GetMethod()))
	}
	fmt.Fprintf(&b, "enums: %d\n", len(fdp.GetEnumType()))
	for _, e := range fdp.GetEnumType() {
		fmt.Fprintf(&b, "  %s (%d values)\n", e.GetName(), len(e.GetValue()))
	}
	return b.String()
}

// LogEntry represents a debug log entry for descriptor operations.
type LogEntry struct {
	Timestamp time.Time
	Operation string
	Target    string
	Detail    string
}

func (e LogEntry) String() string {
	return fmt.Sprintf("[%s] %s %s: %s",
		e.Timestamp.Format("15:04:05.000"), e.Operation, e.Target, e.Detail)
}

// Logger collects debug log entries for descriptor operations.
type Logger struct {
	Entries []LogEntry
}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) Log(op, target, detail string) {
	l.Entries = append(l.Entries, LogEntry{
		Timestamp: time.Now(),
		Operation: op,
		Target:    target,
		Detail:    detail,
	})
}

func (l *Logger) Strings() []string {
	out := make([]string, len(l.Entries))
	for i, e := range l.Entries {
		out[i] = e.String()
	}
	return out
}

// LogFileDescriptor logs a summary of a file descriptor.
func (l *Logger) LogFileDescriptor(fdp *descriptorpb.FileDescriptorProto) {
	l.Log("describe", fdp.GetName(),
		fmt.Sprintf("%d messages, %d services, %d enums",
			len(fdp.GetMessageType()), len(fdp.GetService()), len(fdp.GetEnumType())))
}

// DebugInterceptor returns a gRPC unary server interceptor that logs
// descriptor-related request/response summaries.
func DebugInterceptor(logger *Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		logger.Log("rpc-start", info.FullMethod, describeProtoMessage(req))

		resp, err := handler(ctx, req)

		if err != nil {
			logger.Log("rpc-error", info.FullMethod, err.Error())
		} else {
			logger.Log("rpc-done", info.FullMethod, describeProtoMessage(resp))
		}
		return resp, err
	}
}

// StreamDebugInterceptor returns a gRPC stream server interceptor.
func StreamDebugInterceptor(logger *Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		logger.Log("stream-start", info.FullMethod,
			fmt.Sprintf("client_stream=%v server_stream=%v",
				info.IsClientStream, info.IsServerStream))
		err := handler(srv, ss)
		if err != nil {
			logger.Log("stream-error", info.FullMethod, err.Error())
		} else {
			logger.Log("stream-done", info.FullMethod, "ok")
		}
		return err
	}
}

func describeProtoMessage(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	if m, ok := v.(proto.Message); ok {
		return fmt.Sprintf("%T (size=%d)", m, proto.Size(m))
	}
	return fmt.Sprintf("%T", v)
}

func labelStr(l descriptorpb.FieldDescriptorProto_Label) string {
	switch l {
	case descriptorpb.FieldDescriptorProto_LABEL_REPEATED:
		return "repeated "
	case descriptorpb.FieldDescriptorProto_LABEL_REQUIRED:
		return "required "
	default:
		return ""
	}
}

func typeStr(f *descriptorpb.FieldDescriptorProto) string {
	if f.GetTypeName() != "" {
		return f.GetTypeName()
	}
	switch f.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return "string"
	case descriptorpb.FieldDescriptorProto_TYPE_INT32:
		return "int32"
	case descriptorpb.FieldDescriptorProto_TYPE_INT64:
		return "int64"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32:
		return "uint32"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64:
		return "uint64"
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "bool"
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return "float"
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return "double"
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return "bytes"
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		return f.GetTypeName()
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		return f.GetTypeName()
	default:
		return f.GetType().String()
	}
}
