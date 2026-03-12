// Package server implements the Merger gRPC service.
package server

import (
	"context"
	"fmt"
	"io"

	"github.com/accretional/merge/build"
	"github.com/accretional/merge/descriptor"
	gh "github.com/accretional/merge/github"
	"github.com/accretional/merge/pb"
	protoparse "github.com/accretional/merge/proto"
	"github.com/accretional/merge/walk"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/descriptorpb"
)

// MergerServer implements pb.MergerServer.
type MergerServer struct {
	pb.UnimplementedMergerServer
	GitHubToken string
	Logger      *descriptor.Logger
}

func New(githubToken string) *MergerServer {
	return &MergerServer{
		GitHubToken: githubToken,
		Logger:      descriptor.NewLogger(),
	}
}

// Download receives repo org/user names as a stream of StringValues,
// scans each for .proto files, and streams back FileDescriptorProtos.
func (s *MergerServer) Download(stream grpc.BidiStreamingServer[pb.StringValue, descriptorpb.FileDescriptorProto]) error {
	client := gh.NewClient(s.GitHubToken)

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("receiving: %w", err)
		}

		org := msg.GetValue()
		s.Logger.Log("download", org, "scanning")

		protos, err := client.ScanOrg(org)
		if err != nil {
			s.Logger.Log("download-error", org, err.Error())
			continue
		}
		s.Logger.Log("download", org, fmt.Sprintf("found %d .proto files", len(protos)))

		for _, pf := range protos {
			parsed, err := protoparse.Parse(pf.Content)
			if err != nil {
				s.Logger.Log("parse-error", pf.Repo+"/"+pf.Path, err.Error())
				continue
			}
			fdp := protoparse.FileToDescriptorProto(parsed, pf.Repo+"/"+pf.Path)
			if err := stream.Send(fdp); err != nil {
				return fmt.Errorf("sending %s/%s: %w", pf.Repo, pf.Path, err)
			}
		}
	}
}

// Describe converts any descriptor type to a human-readable StringValue.
func (s *MergerServer) Describe(_ context.Context, req *pb.Descriptor) (*pb.StringValue, error) {
	var result string

	switch {
	case req.GetFile() != nil:
		result = descriptor.DescribeFile(req.GetFile())
	case req.GetMessage() != nil:
		result = descriptor.DescribeMessage(req.GetMessage())
	case req.GetService() != nil:
		result = descriptor.DescribeService(req.GetService())
	case req.GetMethod() != nil:
		result = descriptor.DescribeMethod(req.GetMethod())
	case req.GetField() != nil:
		result = descriptor.DescribeField(req.GetField())
	case req.GetEnum() != nil:
		result = descriptor.DescribeEnum(req.GetEnum())
	default:
		return nil, fmt.Errorf("empty descriptor")
	}

	s.Logger.Log("describe", "descriptor", fmt.Sprintf("%d chars", len(result)))
	return &pb.StringValue{Value: result}, nil
}

// Transform applies walk+transform commands to a file descriptor.
func (s *MergerServer) Transform(_ context.Context, req *pb.WalkRequest) (*pb.WalkResponse, error) {
	if req.GetFile() == nil {
		return nil, fmt.Errorf("missing file descriptor")
	}

	commands := pbCommandsToWalk(req.GetCommands())
	s.Logger.Log("transform", req.GetFile().GetName(),
		fmt.Sprintf("%d commands", len(commands)))

	result, err := walk.Walk(req.GetFile(), commands)
	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}

	return &pb.WalkResponse{
		File: result.File,
		Log:  result.Log,
	}, nil
}

// Build orchestrates the full pipeline: scan repos, parse, transform, bundle.
func (s *MergerServer) Build(_ context.Context, req *pb.BuildRequest) (*pb.BuildResponse, error) {
	commands := pbCommandsToWalk(req.GetCommands())

	s.Logger.Log("build", "pipeline",
		fmt.Sprintf("%d repos, %d commands", len(req.GetSourceRepos()), len(commands)))

	result, err := build.Run(build.Request{
		SourceRepos: req.GetSourceRepos(),
		Commands:    commands,
		GitHubToken: s.GitHubToken,
	})
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}

	return &pb.BuildResponse{
		Files:   result.Files,
		Bundled: result.Bundled,
		Log:     result.Log,
	}, nil
}

// pbCommandsToWalk converts proto TransformCommands to walk.TransformCommands.
func pbCommandsToWalk(cmds []*pb.TransformCommand) []walk.TransformCommand {
	var out []walk.TransformCommand
	for _, cmd := range cmds {
		var wc walk.TransformCommand
		switch {
		case cmd.GetAddField() != nil:
			af := cmd.GetAddField()
			wc.AddField = &walk.FieldModification{
				FieldName:     af.GetFieldName(),
				FieldType:     descriptorpb.FieldDescriptorProto_Type(af.GetFieldType()),
				TypeName:      af.GetTypeName(),
				FieldNumber:   af.GetFieldNumber(),
				Repeated:      af.GetRepeated(),
				TargetMessage: af.GetTargetMessage(),
			}
		case cmd.GetAddMethod() != nil:
			am := cmd.GetAddMethod()
			wc.AddMethod = &walk.ServiceModification{
				MethodName:     am.GetMethodName(),
				InputType:      am.GetInputType(),
				OutputType:     am.GetOutputType(),
				ClientStreaming: am.GetClientStreaming(),
				ServerStreaming: am.GetServerStreaming(),
				TargetService:  am.GetTargetService(),
			}
		case cmd.GetMergeMessages() != nil:
			mm := cmd.GetMergeMessages()
			wc.MergeMessages = &walk.MergeMessages{
				SourceMessage: mm.GetSourceMessage(),
				TargetMessage: mm.GetTargetMessage(),
				ResultName:    mm.GetResultName(),
			}
		case cmd.GetRename() != nil:
			rn := cmd.GetRename()
			wc.Rename = &walk.RenameSymbol{
				OldName: rn.GetOldName(),
				NewName: rn.GetNewName(),
			}
		case cmd.GetRemoveField() != nil:
			rf := cmd.GetRemoveField()
			wc.RemoveField = &walk.RemoveField{
				MessageName: rf.GetMessageName(),
				FieldName:   rf.GetFieldName(),
			}
		}
		out = append(out, wc)
	}
	return out
}
