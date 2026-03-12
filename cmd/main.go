package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/accretional/merge/build"
	mdesc "github.com/accretional/merge/descriptor"
	gh "github.com/accretional/merge/github"
	"github.com/accretional/merge/pb"
	protoparse "github.com/accretional/merge/proto"
	"github.com/accretional/merge/server"
	"github.com/accretional/merge/walk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/descriptorpb"
)

func main() {
	org := flag.String("org", "accretional", "GitHub org/user to scan")
	token := flag.String("token", "", "GitHub API token (or set GITHUB_TOKEN env)")
	outDir := flag.String("out", ".", "Base output directory")
	addUUID := flag.Bool("add-uuid", false, "Add a uuid string field to every message")
	addIdentity := flag.Bool("add-identity", false, "Add Identify/Discover RPCs to every service")
	buildMode := flag.Bool("build", false, "Run full build pipeline with transforms")
	describeMode := flag.Bool("describe", false, "Print descriptor summaries instead of writing files")
	serveMode := flag.Bool("serve", false, "Start gRPC server")
	listenAddr := flag.String("addr", ":9090", "gRPC listen address")
	flag.Parse()

	ghToken := resolveToken(*token)

	// Serve mode: start gRPC server
	if *serveMode {
		runServer(ghToken, *listenAddr)
		return
	}

	// Build transform commands from flags
	commands := buildCommands(*addUUID, *addIdentity)

	// Build mode: use the build pipeline
	if *buildMode {
		runBuild(*org, ghToken, commands)
		return
	}

	// Default mode: scan, download, parse, bundle, split
	runDefault(*org, ghToken, *outDir, *describeMode, commands)
}

func resolveToken(flagToken string) string {
	if flagToken != "" {
		return flagToken
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	if t := gh.TokenFromGHCLI(); t != "" {
		fmt.Println("Using token from gh CLI auth")
		return t
	}
	fmt.Fprintln(os.Stderr, "warning: no auth token — private repos will not be visible")
	return ""
}

func buildCommands(addUUID, addIdentity bool) []walk.TransformCommand {
	var commands []walk.TransformCommand
	if addUUID {
		commands = append(commands, walk.TransformCommand{
			AddField: &walk.FieldModification{
				FieldName: "uuid",
				FieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING,
			},
		})
	}
	if addIdentity {
		commands = append(commands, walk.TransformCommand{
			AddMethod: &walk.ServiceModification{
				MethodName: "Identify",
				InputType:  "StringValue",
				OutputType: "StringValue",
			},
		})
		commands = append(commands, walk.TransformCommand{
			AddMethod: &walk.ServiceModification{
				MethodName: "Discover",
				InputType:  "StringValue",
				OutputType: "StringValue",
			},
		})
	}
	return commands
}

func runServer(ghToken, addr string) {
	logger := mdesc.NewLogger()

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(mdesc.DebugInterceptor(logger)),
		grpc.ChainStreamInterceptor(mdesc.StreamDebugInterceptor(logger)),
	)

	srv := server.New(ghToken)
	pb.RegisterMergerServer(grpcServer, srv)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Merger gRPC server listening on %s\n", addr)
	if err := grpcServer.Serve(lis); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(1)
	}
}

func runBuild(org, ghToken string, commands []walk.TransformCommand) {
	orgs := strings.Split(org, ",")
	result, err := build.Run(build.Request{
		SourceRepos: orgs,
		Commands:    commands,
		GitHubToken: ghToken,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Build complete: %d files, bundle has %d messages, %d services\n",
		len(result.Files),
		len(result.Bundled.GetMessageType()),
		len(result.Bundled.GetService()))
	for _, entry := range result.Log {
		fmt.Printf("  %s\n", entry)
	}
}

func runDefault(org, ghToken, outDir string, describeMode bool, commands []walk.TransformCommand) {
	client := gh.NewClient(ghToken)

	fmt.Printf("Scanning %s for .proto files...\n", org)
	protos, err := client.ScanOrg(org)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error scanning org: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Found %d .proto files\n", len(protos))

	if len(protos) == 0 {
		fmt.Println("No .proto files found.")
		return
	}

	// Save to proto-download/
	downloadDir := filepath.Join(outDir, "proto-download")
	for _, pf := range protos {
		dir := filepath.Join(downloadDir, pf.Repo, filepath.Dir(pf.Path))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", dir, err)
			continue
		}
		outPath := filepath.Join(downloadDir, pf.Repo, pf.Path)
		if err := os.WriteFile(outPath, []byte(pf.Content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", outPath, err)
			continue
		}
		fmt.Printf("  downloaded: %s/%s\n", pf.Repo, pf.Path)
	}

	// Parse
	var parsed []*protoparse.File
	for _, pf := range protos {
		f, err := protoparse.Parse(pf.Content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  parse error %s/%s: %v\n", pf.Repo, pf.Path, err)
			continue
		}
		parsed = append(parsed, f)
	}
	fmt.Printf("Parsed %d files\n", len(parsed))

	// Describe mode
	if describeMode {
		for _, pf := range protos {
			f, _ := protoparse.Parse(pf.Content)
			if f == nil {
				continue
			}
			fmt.Printf("\n--- %s/%s ---\n", pf.Repo, pf.Path)
			fmt.Printf("package: %s\n", f.Package)
			fmt.Printf("messages: %d\n", len(f.Messages))
			for _, m := range f.Messages {
				fmt.Printf("  %s\n", mdesc.DescribeMessage(protoparse.MessageToDescriptorProto(m)))
			}
			fmt.Printf("services: %d\n", len(f.Services))
			for _, s := range f.Services {
				fmt.Printf("  %s\n", mdesc.DescribeService(protoparse.ServiceToDescriptorProto(s, f.Package)))
			}
		}
		return
	}

	// Bundle
	bundleDir := filepath.Join(outDir, "proto-bundle")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir bundle: %v\n", err)
		os.Exit(1)
	}
	bundled := protoparse.Bundle(parsed)
	bundlePath := filepath.Join(bundleDir, "bundle.proto")
	if err := os.WriteFile(bundlePath, []byte(bundled), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write bundle: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Bundled all types into %s\n", bundlePath)

	// Split
	splitBaseDir := filepath.Join(outDir, "proto-split")
	splits := protoparse.Split(parsed)
	for _, s := range splits {
		dir := filepath.Join(splitBaseDir, s.Dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir split %s: %v\n", dir, err)
			continue
		}
		outPath := filepath.Join(dir, s.Filename)
		if err := os.WriteFile(outPath, []byte(s.Content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write split %s: %v\n", outPath, err)
			continue
		}
	}
	fmt.Printf("Split into %d files under %s\n", len(splits), splitBaseDir)

	fmt.Println("Done.")
}
