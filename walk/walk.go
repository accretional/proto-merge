// Package walk provides a Walk-like operation over FileDescriptorProtos,
// applying TransformCommands to each message/service encountered during traversal.
package walk

import (
	"fmt"

	"github.com/accretional/merge/descriptor"
	"github.com/accretional/merge/transform"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// TransformCommand mirrors the proto definition for use in Go code.
type TransformCommand struct {
	// Exactly one of these should be set
	AddField      *FieldModification
	AddMethod     *ServiceModification
	MergeMessages *MergeMessages
	Rename        *RenameSymbol
	RemoveField   *RemoveField
}

type FieldModification struct {
	FieldName     string
	FieldType     descriptorpb.FieldDescriptorProto_Type
	TypeName      string
	FieldNumber   int32
	Repeated      bool
	TargetMessage string // empty = all messages
}

type ServiceModification struct {
	MethodName      string
	InputType       string
	OutputType      string
	ClientStreaming  bool
	ServerStreaming  bool
	TargetService   string // empty = all services
}

type MergeMessages struct {
	SourceMessage string
	TargetMessage string
	ResultName    string
}

type RenameSymbol struct {
	OldName string
	NewName string
}

type RemoveField struct {
	MessageName string
	FieldName   string
}

// WalkResult contains the output of a Walk operation.
type WalkResult struct {
	File *descriptorpb.FileDescriptorProto
	Log  []string
}

// Walk traverses a FileDescriptorProto, visiting each message and service,
// and applies the given transform commands. This is analogous to protomessage.Walk
// but operates at the descriptor level and applies transformations.
func Walk(fdp *descriptorpb.FileDescriptorProto, commands []TransformCommand) (*WalkResult, error) {
	if fdp == nil {
		return nil, fmt.Errorf("nil file descriptor")
	}

	logger := descriptor.NewLogger()
	logger.LogFileDescriptor(fdp)

	// Clone so we don't mutate the input
	result := proto.Clone(fdp).(*descriptorpb.FileDescriptorProto)

	// Walk messages
	for _, msg := range result.GetMessageType() {
		walkMessage(msg, "", logger)
	}

	// Walk services
	for _, svc := range result.GetService() {
		walkService(svc, logger)
	}

	// Walk enums
	for _, e := range result.GetEnumType() {
		logger.Log("visit-enum", e.GetName(),
			fmt.Sprintf("%d values", len(e.GetValue())))
	}

	// Apply transforms
	var err error
	for i, cmd := range commands {
		result, err = applyCommand(result, cmd, logger)
		if err != nil {
			return nil, fmt.Errorf("command %d: %w", i, err)
		}
	}

	logger.Log("walk-complete", fdp.GetName(),
		fmt.Sprintf("output: %d messages, %d services",
			len(result.GetMessageType()), len(result.GetService())))

	return &WalkResult{
		File: result,
		Log:  logger.Strings(),
	}, nil
}

// WalkMessages calls visitor for each message (including nested) in the file.
func WalkMessages(fdp *descriptorpb.FileDescriptorProto, visitor func(path string, msg *descriptorpb.DescriptorProto) error) error {
	for _, msg := range fdp.GetMessageType() {
		if err := walkMessageVisitor(msg, "", visitor); err != nil {
			return err
		}
	}
	return nil
}

// WalkServices calls visitor for each service in the file.
func WalkServices(fdp *descriptorpb.FileDescriptorProto, visitor func(svc *descriptorpb.ServiceDescriptorProto) error) error {
	for _, svc := range fdp.GetService() {
		if err := visitor(svc); err != nil {
			return err
		}
	}
	return nil
}

func walkMessageVisitor(msg *descriptorpb.DescriptorProto, prefix string, visitor func(string, *descriptorpb.DescriptorProto) error) error {
	path := prefix + msg.GetName()
	if err := visitor(path, msg); err != nil {
		return err
	}
	for _, nested := range msg.GetNestedType() {
		if err := walkMessageVisitor(nested, path+".", visitor); err != nil {
			return err
		}
	}
	return nil
}

func walkMessage(msg *descriptorpb.DescriptorProto, prefix string, logger *descriptor.Logger) {
	path := prefix + msg.GetName()
	logger.Log("visit-message", path,
		fmt.Sprintf("%d fields, %d nested, %d oneofs",
			len(msg.GetField()), len(msg.GetNestedType()), len(msg.GetOneofDecl())))

	for _, f := range msg.GetField() {
		logger.Log("visit-field", path+"."+f.GetName(),
			fmt.Sprintf("type=%s number=%d", f.GetType(), f.GetNumber()))
	}
	for _, nested := range msg.GetNestedType() {
		walkMessage(nested, path+".", logger)
	}
}

func walkService(svc *descriptorpb.ServiceDescriptorProto, logger *descriptor.Logger) {
	logger.Log("visit-service", svc.GetName(),
		fmt.Sprintf("%d methods", len(svc.GetMethod())))
	for _, m := range svc.GetMethod() {
		logger.Log("visit-method", svc.GetName()+"."+m.GetName(),
			fmt.Sprintf("input=%s output=%s client_stream=%v server_stream=%v",
				m.GetInputType(), m.GetOutputType(),
				m.GetClientStreaming(), m.GetServerStreaming()))
	}
}

func applyCommand(fdp *descriptorpb.FileDescriptorProto, cmd TransformCommand, logger *descriptor.Logger) (*descriptorpb.FileDescriptorProto, error) {
	switch {
	case cmd.AddField != nil:
		af := cmd.AddField
		logger.Log("transform-add-field", af.TargetMessage,
			fmt.Sprintf("field=%s type=%s", af.FieldName, af.FieldType))
		return transform.AddFieldToMessage(fdp, af.TargetMessage, af.FieldName, af.FieldType, af.TypeName, af.FieldNumber, af.Repeated)

	case cmd.AddMethod != nil:
		am := cmd.AddMethod
		logger.Log("transform-add-method", am.TargetService,
			fmt.Sprintf("method=%s input=%s output=%s", am.MethodName, am.InputType, am.OutputType))
		return transform.AddMethodToService(fdp, am.TargetService, am.MethodName, am.InputType, am.OutputType, am.ClientStreaming, am.ServerStreaming)

	case cmd.MergeMessages != nil:
		mm := cmd.MergeMessages
		logger.Log("transform-merge", mm.TargetMessage,
			fmt.Sprintf("source=%s result=%s", mm.SourceMessage, mm.ResultName))
		return transform.MergeMessagesInFile(fdp, mm.SourceMessage, mm.TargetMessage, mm.ResultName)

	case cmd.Rename != nil:
		rn := cmd.Rename
		logger.Log("transform-rename", rn.OldName, fmt.Sprintf("new=%s", rn.NewName))
		return transform.RenameSymbolInFile(fdp, rn.OldName, rn.NewName)

	case cmd.RemoveField != nil:
		rf := cmd.RemoveField
		logger.Log("transform-remove-field", rf.MessageName, fmt.Sprintf("field=%s", rf.FieldName))
		return transform.RemoveFieldFromMessage(fdp, rf.MessageName, rf.FieldName)

	default:
		return fdp, nil
	}
}
