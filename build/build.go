// Package build orchestrates the full pipeline: scan GitHub repos for protos,
// parse them into descriptors, apply transformations, and produce output.
package build

import (
	"fmt"

	"github.com/accretional/merge/descriptor"
	gh "github.com/accretional/merge/github"
	protoparse "github.com/accretional/merge/proto"
	"github.com/accretional/merge/transform"
	"github.com/accretional/merge/walk"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Request describes a Build operation.
type Request struct {
	SourceRepos []string              // GitHub org/user names to scan
	Commands    []walk.TransformCommand // Transformations to apply to each file
	GitHubToken string                 // Optional GitHub token
}

// Result contains the output of a Build operation.
type Result struct {
	Files   []*descriptorpb.FileDescriptorProto
	Bundled *descriptorpb.FileDescriptorProto
	Log     []string
}

// Run executes the full build pipeline.
func Run(req Request) (*Result, error) {
	logger := descriptor.NewLogger()
	logger.Log("build-start", "pipeline", fmt.Sprintf("%d sources, %d commands", len(req.SourceRepos), len(req.Commands)))

	// Step 1: Scan and download .proto files from all sources
	var allProtos []gh.ProtoFile
	client := gh.NewClient(req.GitHubToken)

	for _, repo := range req.SourceRepos {
		logger.Log("scan", repo, "scanning for .proto files")
		protos, err := client.ScanOrg(repo)
		if err != nil {
			logger.Log("scan-error", repo, err.Error())
			continue
		}
		logger.Log("scan-done", repo, fmt.Sprintf("found %d .proto files", len(protos)))
		allProtos = append(allProtos, protos...)
	}

	if len(allProtos) == 0 {
		return nil, fmt.Errorf("no .proto files found in sources: %v", req.SourceRepos)
	}

	// Step 2: Parse each proto file into a FileDescriptorProto
	var files []*descriptorpb.FileDescriptorProto
	for _, pf := range allProtos {
		parsed, err := protoparse.Parse(pf.Content)
		if err != nil {
			logger.Log("parse-error", pf.Repo+"/"+pf.Path, err.Error())
			continue
		}
		fdp := parsedToFileDescriptor(pf, parsed)
		files = append(files, fdp)
		logger.Log("parsed", pf.Repo+"/"+pf.Path,
			fmt.Sprintf("%d messages, %d services",
				len(fdp.GetMessageType()), len(fdp.GetService())))
	}

	// Step 3: Apply transformations to each file via Walk
	if len(req.Commands) > 0 {
		logger.Log("transform", "pipeline", fmt.Sprintf("applying %d commands to %d files", len(req.Commands), len(files)))
		var transformed []*descriptorpb.FileDescriptorProto
		for _, fdp := range files {
			result, err := walk.Walk(fdp, req.Commands)
			if err != nil {
				logger.Log("transform-error", fdp.GetName(), err.Error())
				// Keep original on transform failure
				transformed = append(transformed, fdp)
				continue
			}
			transformed = append(transformed, result.File)
			logger.Log("transformed", fdp.GetName(), "ok")
		}
		files = transformed
	}

	// Step 4: Bundle all files into one
	bundled, err := transform.CombineFiles(files, "bundle.proto", "bundle")
	if err != nil {
		return nil, fmt.Errorf("bundling: %w", err)
	}
	logger.Log("bundled", "bundle.proto",
		fmt.Sprintf("%d messages, %d services, %d enums",
			len(bundled.GetMessageType()), len(bundled.GetService()), len(bundled.GetEnumType())))

	return &Result{
		Files:   files,
		Bundled: bundled,
		Log:     logger.Strings(),
	}, nil
}

// RunLocal runs the build pipeline on already-parsed file descriptors (no GitHub scan).
func RunLocal(files []*descriptorpb.FileDescriptorProto, commands []walk.TransformCommand) (*Result, error) {
	logger := descriptor.NewLogger()
	logger.Log("build-local", "pipeline", fmt.Sprintf("%d files, %d commands", len(files), len(commands)))

	// Apply transforms
	if len(commands) > 0 {
		var transformed []*descriptorpb.FileDescriptorProto
		for _, fdp := range files {
			result, err := walk.Walk(fdp, commands)
			if err != nil {
				logger.Log("transform-error", fdp.GetName(), err.Error())
				transformed = append(transformed, fdp)
				continue
			}
			transformed = append(transformed, result.File)
		}
		files = transformed
	}

	bundled, err := transform.CombineFiles(files, "bundle.proto", "bundle")
	if err != nil {
		return nil, fmt.Errorf("bundling: %w", err)
	}

	return &Result{
		Files:   files,
		Bundled: bundled,
		Log:     logger.Strings(),
	}, nil
}

// parsedToFileDescriptor converts our simple parsed proto to a FileDescriptorProto.
func parsedToFileDescriptor(pf gh.ProtoFile, parsed *protoparse.File) *descriptorpb.FileDescriptorProto {
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String(pf.Repo + "/" + pf.Path),
		Package: proto.String(parsed.Package),
		Syntax:  proto.String(parsed.Syntax),
	}
	if fdp.GetSyntax() == "" {
		fdp.Syntax = proto.String("proto3")
	}

	for _, imp := range parsed.Imports {
		fdp.Dependency = append(fdp.Dependency, imp)
	}

	for _, msg := range parsed.Messages {
		fdp.MessageType = append(fdp.MessageType, parseMessageToDescriptor(msg))
	}

	for _, svc := range parsed.Services {
		fdp.Service = append(fdp.Service, parseServiceToDescriptor(svc, parsed.Package))
	}

	for _, e := range parsed.Enums {
		fdp.EnumType = append(fdp.EnumType, parseEnumToDescriptor(e))
	}

	return fdp
}

func parseMessageToDescriptor(msg protoparse.Message) *descriptorpb.DescriptorProto {
	return &descriptorpb.DescriptorProto{
		Name: proto.String(msg.Name),
	}
}

func parseServiceToDescriptor(svc protoparse.Service, pkg string) *descriptorpb.ServiceDescriptorProto {
	sd := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String(svc.Name),
	}
	// We don't deeply parse methods from the text-based parser,
	// but we create a placeholder structure
	return sd
}

func parseEnumToDescriptor(e protoparse.Enum) *descriptorpb.EnumDescriptorProto {
	return &descriptorpb.EnumDescriptorProto{
		Name: proto.String(e.Name),
	}
}
