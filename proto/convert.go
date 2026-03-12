package proto

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// MessageToDescriptorProto converts our parsed Message to a protobuf DescriptorProto.
func MessageToDescriptorProto(m Message) *descriptorpb.DescriptorProto {
	return &descriptorpb.DescriptorProto{
		Name: proto.String(m.Name),
	}
}

// ServiceToDescriptorProto converts our parsed Service to a protobuf ServiceDescriptorProto.
func ServiceToDescriptorProto(s Service, pkg string) *descriptorpb.ServiceDescriptorProto {
	return &descriptorpb.ServiceDescriptorProto{
		Name: proto.String(s.Name),
	}
}

// EnumToDescriptorProto converts our parsed Enum to a protobuf EnumDescriptorProto.
func EnumToDescriptorProto(e Enum) *descriptorpb.EnumDescriptorProto {
	return &descriptorpb.EnumDescriptorProto{
		Name: proto.String(e.Name),
	}
}

// FileToDescriptorProto converts our parsed File to a FileDescriptorProto.
func FileToDescriptorProto(f *File, name string) *descriptorpb.FileDescriptorProto {
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String(name),
		Package: proto.String(f.Package),
		Syntax:  proto.String(f.Syntax),
	}
	if fdp.GetSyntax() == "" {
		fdp.Syntax = proto.String("proto3")
	}
	for _, imp := range f.Imports {
		fdp.Dependency = append(fdp.Dependency, imp)
	}
	for _, msg := range f.Messages {
		fdp.MessageType = append(fdp.MessageType, MessageToDescriptorProto(msg))
	}
	for _, svc := range f.Services {
		fdp.Service = append(fdp.Service, ServiceToDescriptorProto(svc, f.Package))
	}
	for _, e := range f.Enums {
		fdp.EnumType = append(fdp.EnumType, EnumToDescriptorProto(e))
	}
	return fdp
}
