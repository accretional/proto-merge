// Package transform provides programmatic composition and modification of
// protobuf descriptors using jhump/protoreflect builders.
package transform

import (
	"fmt"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/builder"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// AddFieldToMessage adds a new field to a message within a file descriptor.
// If messageName is empty, the field is added to all top-level messages.
func AddFieldToMessage(
	fdp *descriptorpb.FileDescriptorProto,
	messageName string,
	fieldName string,
	fieldType descriptorpb.FieldDescriptorProto_Type,
	typeName string,
	fieldNumber int32,
	repeated bool,
) (*descriptorpb.FileDescriptorProto, error) {
	fd, err := desc.CreateFileDescriptor(fdp)
	if err != nil {
		return nil, fmt.Errorf("creating descriptor: %w", err)
	}
	fb, err := builder.FromFile(fd)
	if err != nil {
		return nil, fmt.Errorf("creating builder: %w", err)
	}

	msgs := fb.GetChildren()
	for _, child := range msgs {
		mb, ok := child.(*builder.MessageBuilder)
		if !ok {
			continue
		}
		if messageName != "" && mb.GetName() != messageName {
			continue
		}
		if err := addFieldToBuilder(mb, fieldName, fieldType, typeName, fieldNumber, repeated, fb); err != nil {
			return nil, fmt.Errorf("adding field to %s: %w", mb.GetName(), err)
		}
	}

	newFd, err := fb.Build()
	if err != nil {
		return nil, fmt.Errorf("building: %w", err)
	}
	return newFd.AsFileDescriptorProto(), nil
}

func addFieldToBuilder(
	mb *builder.MessageBuilder,
	fieldName string,
	fieldType descriptorpb.FieldDescriptorProto_Type,
	typeName string,
	fieldNumber int32,
	repeated bool,
	fb *builder.FileBuilder,
) error {
	var ft *builder.FieldType
	switch {
	case typeName != "" && (fieldType == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE ||
		fieldType == descriptorpb.FieldDescriptorProto_TYPE_ENUM):
		// Look up the referenced type in the file
		if fieldType == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
			refMb := findMessage(fb, typeName)
			if refMb != nil {
				ft = builder.FieldTypeMessage(refMb)
			} else {
				ft = builder.FieldTypeString()
			}
		} else {
			refEb := findEnum(fb, typeName)
			if refEb != nil {
				ft = builder.FieldTypeEnum(refEb)
			} else {
				ft = builder.FieldTypeInt32()
			}
		}
	default:
		ft = scalarFieldType(fieldType)
	}

	flb := builder.NewField(fieldName, ft)
	if fieldNumber > 0 {
		flb.SetNumber(fieldNumber)
	}
	if repeated {
		flb.SetRepeated()
	}
	return mb.TryAddField(flb)
}

// AddMethodToService adds a new RPC method to a service in a file descriptor.
// If serviceName is empty, the method is added to all services.
func AddMethodToService(
	fdp *descriptorpb.FileDescriptorProto,
	serviceName string,
	methodName string,
	inputType string,
	outputType string,
	clientStreaming bool,
	serverStreaming bool,
) (*descriptorpb.FileDescriptorProto, error) {
	fd, err := desc.CreateFileDescriptor(fdp)
	if err != nil {
		return nil, fmt.Errorf("creating descriptor: %w", err)
	}
	fb, err := builder.FromFile(fd)
	if err != nil {
		return nil, fmt.Errorf("creating builder: %w", err)
	}

	// Find input/output message builders
	inputMb := findMessage(fb, inputType)
	outputMb := findMessage(fb, outputType)

	// If not found, create stub messages
	if inputMb == nil {
		inputMb = builder.NewMessage(inputType)
		fb.AddMessage(inputMb)
	}
	if outputMb == nil && outputType != inputType {
		outputMb = builder.NewMessage(outputType)
		fb.AddMessage(outputMb)
	} else if outputMb == nil {
		outputMb = inputMb
	}

	inputRpc := builder.RpcTypeMessage(inputMb, clientStreaming)
	outputRpc := builder.RpcTypeMessage(outputMb, serverStreaming)
	mtb := builder.NewMethod(methodName, inputRpc, outputRpc)

	for _, child := range fb.GetChildren() {
		sb, ok := child.(*builder.ServiceBuilder)
		if !ok {
			continue
		}
		if serviceName != "" && sb.GetName() != serviceName {
			continue
		}
		if err := sb.TryAddMethod(mtb); err != nil {
			return nil, fmt.Errorf("adding method to %s: %w", sb.GetName(), err)
		}
	}

	newFd, err := fb.Build()
	if err != nil {
		return nil, fmt.Errorf("building: %w", err)
	}
	return newFd.AsFileDescriptorProto(), nil
}

// MergeMessagesInFile merges fields from sourceMessage into targetMessage.
// If resultName is non-empty, the target message is renamed.
func MergeMessagesInFile(
	fdp *descriptorpb.FileDescriptorProto,
	sourceMessage string,
	targetMessage string,
	resultName string,
) (*descriptorpb.FileDescriptorProto, error) {
	fd, err := desc.CreateFileDescriptor(fdp)
	if err != nil {
		return nil, fmt.Errorf("creating descriptor: %w", err)
	}
	fb, err := builder.FromFile(fd)
	if err != nil {
		return nil, fmt.Errorf("creating builder: %w", err)
	}

	srcMb := findMessage(fb, sourceMessage)
	tgtMb := findMessage(fb, targetMessage)
	if srcMb == nil {
		return nil, fmt.Errorf("source message %q not found", sourceMessage)
	}
	if tgtMb == nil {
		return nil, fmt.Errorf("target message %q not found", targetMessage)
	}

	// Copy fields from source to target
	for _, child := range srcMb.GetChildren() {
		srcField, ok := child.(*builder.FieldBuilder)
		if !ok {
			continue
		}
		// Skip if target already has this field
		if tgtMb.GetField(srcField.GetName()) != nil {
			continue
		}
		// Clone the field by re-creating it
		newField := builder.NewField(srcField.GetName(), srcField.GetType())
		newField.SetNumber(srcField.GetNumber())
		if srcField.IsRepeated() {
			newField.SetRepeated()
		}
		if err := tgtMb.TryAddField(newField); err != nil {
			// Field number conflict — auto-assign
			newField.SetNumber(0)
			if err := tgtMb.TryAddField(newField); err != nil {
				return nil, fmt.Errorf("merging field %s: %w", srcField.GetName(), err)
			}
		}
	}

	if resultName != "" {
		tgtMb.SetName(resultName)
	}

	newFd, err := fb.Build()
	if err != nil {
		return nil, fmt.Errorf("building: %w", err)
	}
	return newFd.AsFileDescriptorProto(), nil
}

// RenameSymbolInFile renames a message, service, or enum.
func RenameSymbolInFile(
	fdp *descriptorpb.FileDescriptorProto,
	oldName string,
	newName string,
) (*descriptorpb.FileDescriptorProto, error) {
	fd, err := desc.CreateFileDescriptor(fdp)
	if err != nil {
		return nil, fmt.Errorf("creating descriptor: %w", err)
	}
	fb, err := builder.FromFile(fd)
	if err != nil {
		return nil, fmt.Errorf("creating builder: %w", err)
	}

	found := false
	for _, child := range fb.GetChildren() {
		switch b := child.(type) {
		case *builder.MessageBuilder:
			if b.GetName() == oldName {
				b.SetName(newName)
				found = true
			}
		case *builder.ServiceBuilder:
			if b.GetName() == oldName {
				b.SetName(newName)
				found = true
			}
		case *builder.EnumBuilder:
			if b.GetName() == oldName {
				b.SetName(newName)
				found = true
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("symbol %q not found", oldName)
	}

	newFd, err := fb.Build()
	if err != nil {
		return nil, fmt.Errorf("building: %w", err)
	}
	return newFd.AsFileDescriptorProto(), nil
}

// RemoveFieldFromMessage removes a field by name from a message.
func RemoveFieldFromMessage(
	fdp *descriptorpb.FileDescriptorProto,
	messageName string,
	fieldName string,
) (*descriptorpb.FileDescriptorProto, error) {
	fd, err := desc.CreateFileDescriptor(fdp)
	if err != nil {
		return nil, fmt.Errorf("creating descriptor: %w", err)
	}
	fb, err := builder.FromFile(fd)
	if err != nil {
		return nil, fmt.Errorf("creating builder: %w", err)
	}

	found := false
	for _, child := range fb.GetChildren() {
		mb, ok := child.(*builder.MessageBuilder)
		if !ok || (messageName != "" && mb.GetName() != messageName) {
			continue
		}
		if mb.GetField(fieldName) != nil {
			mb.RemoveField(fieldName)
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("field %q not found in %q", fieldName, messageName)
	}

	newFd, err := fb.Build()
	if err != nil {
		return nil, fmt.Errorf("building: %w", err)
	}
	return newFd.AsFileDescriptorProto(), nil
}

// CombineFiles merges multiple FileDescriptorProtos into one.
// Messages, services, and enums are all combined. Duplicates by name are skipped.
func CombineFiles(files []*descriptorpb.FileDescriptorProto, resultName, pkg string) (*descriptorpb.FileDescriptorProto, error) {
	result := &descriptorpb.FileDescriptorProto{
		Name:    proto.String(resultName),
		Package: proto.String(pkg),
		Syntax:  proto.String("proto3"),
	}

	msgSeen := map[string]bool{}
	svcSeen := map[string]bool{}
	enumSeen := map[string]bool{}

	for _, f := range files {
		for _, m := range f.GetMessageType() {
			if !msgSeen[m.GetName()] {
				msgSeen[m.GetName()] = true
				result.MessageType = append(result.MessageType, proto.Clone(m).(*descriptorpb.DescriptorProto))
			}
		}
		for _, s := range f.GetService() {
			if !svcSeen[s.GetName()] {
				svcSeen[s.GetName()] = true
				result.Service = append(result.Service, proto.Clone(s).(*descriptorpb.ServiceDescriptorProto))
			}
		}
		for _, e := range f.GetEnumType() {
			if !enumSeen[e.GetName()] {
				enumSeen[e.GetName()] = true
				result.EnumType = append(result.EnumType, proto.Clone(e).(*descriptorpb.EnumDescriptorProto))
			}
		}
	}

	return result, nil
}

// NewFileDescriptor creates a minimal FileDescriptorProto.
func NewFileDescriptor(name, pkg string) *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String(name),
		Package: proto.String(pkg),
		Syntax:  proto.String("proto3"),
	}
}

// NewMessage creates a new DescriptorProto with the given fields.
func NewMessage(name string, fields map[string]descriptorpb.FieldDescriptorProto_Type) *descriptorpb.DescriptorProto {
	msg := &descriptorpb.DescriptorProto{
		Name: proto.String(name),
	}
	num := int32(1)
	for fname, ftype := range fields {
		ft := ftype
		msg.Field = append(msg.Field, &descriptorpb.FieldDescriptorProto{
			Name:   proto.String(fname),
			Number: proto.Int32(num),
			Type:   &ft,
			Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		})
		num++
	}
	return msg
}

// NewService creates a new ServiceDescriptorProto.
func NewService(name string, methods []*descriptorpb.MethodDescriptorProto) *descriptorpb.ServiceDescriptorProto {
	return &descriptorpb.ServiceDescriptorProto{
		Name:   proto.String(name),
		Method: methods,
	}
}

// NewMethod creates a new MethodDescriptorProto.
func NewMethod(name, inputType, outputType string, clientStream, serverStream bool) *descriptorpb.MethodDescriptorProto {
	return &descriptorpb.MethodDescriptorProto{
		Name:            proto.String(name),
		InputType:       proto.String(inputType),
		OutputType:      proto.String(outputType),
		ClientStreaming: proto.Bool(clientStream),
		ServerStreaming: proto.Bool(serverStream),
	}
}

func findMessage(fb *builder.FileBuilder, name string) *builder.MessageBuilder {
	// Strip leading dot and package prefix for matching
	cleanName := name
	if idx := strings.LastIndex(cleanName, "."); idx >= 0 {
		cleanName = cleanName[idx+1:]
	}
	for _, child := range fb.GetChildren() {
		mb, ok := child.(*builder.MessageBuilder)
		if ok && mb.GetName() == cleanName {
			return mb
		}
	}
	return nil
}

func findEnum(fb *builder.FileBuilder, name string) *builder.EnumBuilder {
	cleanName := name
	if idx := strings.LastIndex(cleanName, "."); idx >= 0 {
		cleanName = cleanName[idx+1:]
	}
	for _, child := range fb.GetChildren() {
		eb, ok := child.(*builder.EnumBuilder)
		if ok && eb.GetName() == cleanName {
			return eb
		}
	}
	return nil
}

func scalarFieldType(t descriptorpb.FieldDescriptorProto_Type) *builder.FieldType {
	switch t {
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return builder.FieldTypeString()
	case descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_TYPE_SINT32:
		return builder.FieldTypeInt32()
	case descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_TYPE_SINT64:
		return builder.FieldTypeInt64()
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32:
		return builder.FieldTypeUInt32()
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64:
		return builder.FieldTypeUInt64()
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return builder.FieldTypeBool()
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return builder.FieldTypeFloat()
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return builder.FieldTypeDouble()
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return builder.FieldTypeBytes()
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		return builder.FieldTypeFixed32()
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		return builder.FieldTypeFixed64()
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		return builder.FieldTypeSFixed32()
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return builder.FieldTypeSFixed64()
	default:
		return builder.FieldTypeString()
	}
}
