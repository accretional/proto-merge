package proto

import (
	"fmt"
	"strings"
)

// SplitResult represents a single output file from the split operation.
type SplitResult struct {
	// Dir is the package directory (e.g., "mypackage")
	Dir string
	// Filename within the directory
	Filename string
	// Content is the full .proto file content
	Content string
}

// Split takes parsed proto files and produces one file per message and service,
// organized into package directories.
func Split(files []*File) []SplitResult {
	var results []SplitResult

	// Track what we've emitted to deduplicate
	seen := map[string]bool{}

	for _, f := range files {
		pkg := f.Package
		if pkg == "" {
			pkg = "default"
		}
		dir := strings.ReplaceAll(pkg, ".", "/")
		syntax := f.Syntax
		if syntax == "" {
			syntax = "proto3"
		}

		for _, e := range f.Enums {
			key := dir + "/" + e.Name
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, SplitResult{
				Dir:      dir,
				Filename: toSnakeCase(e.Name) + ".proto",
				Content:  makeHeader(syntax, pkg) + e.Body + "\n",
			})
		}

		for _, m := range f.Messages {
			key := dir + "/" + m.Name
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, SplitResult{
				Dir:      dir,
				Filename: toSnakeCase(m.Name) + ".proto",
				Content:  makeHeader(syntax, pkg) + m.Body + "\n",
			})
		}

		for _, s := range f.Services {
			key := dir + "/" + s.Name
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, SplitResult{
				Dir:      dir,
				Filename: toSnakeCase(s.Name) + "_service.proto",
				Content:  makeHeader(syntax, pkg) + s.Body + "\n",
			})
		}
	}

	return results
}

func makeHeader(syntax, pkg string) string {
	return fmt.Sprintf("syntax = \"%s\";\n\npackage %s;\n\n", syntax, pkg)
}

func toSnakeCase(name string) string {
	var b strings.Builder
	for i, r := range name {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
