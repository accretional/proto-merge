// Package proto provides utilities for parsing, bundling, and splitting
// protobuf source files.
package proto

import (
	"fmt"
	"regexp"
	"strings"
)

// File represents a parsed .proto file.
type File struct {
	Syntax   string
	Package  string
	Options  []string
	Imports  []string
	Messages []Message
	Services []Service
	Enums    []Enum
	Raw      string
}

type Message struct {
	Name string
	Body string // full block including nested types
}

type Service struct {
	Name string
	Body string // full block including RPCs
}

type Enum struct {
	Name string
	Body string
}

var (
	syntaxRe  = regexp.MustCompile(`(?m)^syntax\s*=\s*"([^"]+)"\s*;`)
	packageRe = regexp.MustCompile(`(?m)^package\s+([\w.]+)\s*;`)
	optionRe  = regexp.MustCompile(`(?m)^option\s+.+;`)
	importRe  = regexp.MustCompile(`(?m)^import\s+"([^"]+)"\s*;`)
)

// Parse extracts structure from a .proto file's source text.
func Parse(source string) (*File, error) {
	f := &File{Raw: source}

	if m := syntaxRe.FindStringSubmatch(source); len(m) > 1 {
		f.Syntax = m[1]
	}
	if m := packageRe.FindStringSubmatch(source); len(m) > 1 {
		f.Package = m[1]
	}
	for _, m := range optionRe.FindAllString(source, -1) {
		f.Options = append(f.Options, m)
	}
	for _, m := range importRe.FindAllStringSubmatch(source, -1) {
		if len(m) > 1 {
			f.Imports = append(f.Imports, m[1])
		}
	}

	f.Messages = extractBlocks(source, "message")
	f.Services = extractServiceBlocks(source)
	f.Enums = extractEnumBlocks(source)

	return f, nil
}

// extractBlocks pulls top-level named blocks (message, enum, etc.) from proto source.
func extractBlocks(source, keyword string) []Message {
	var results []Message
	re := regexp.MustCompile(fmt.Sprintf(`(?m)^%s\s+(\w+)\s*\{`, keyword))
	matches := re.FindAllStringSubmatchIndex(source, -1)
	for _, loc := range matches {
		name := source[loc[2]:loc[3]]
		braceStart := strings.Index(source[loc[0]:], "{") + loc[0]
		body := extractBraceBlock(source, braceStart)
		results = append(results, Message{
			Name: name,
			Body: source[loc[0] : braceStart+len(body)+1],
		})
	}
	return results
}

func extractServiceBlocks(source string) []Service {
	var results []Service
	re := regexp.MustCompile(`(?m)^service\s+(\w+)\s*\{`)
	matches := re.FindAllStringSubmatchIndex(source, -1)
	for _, loc := range matches {
		name := source[loc[2]:loc[3]]
		braceStart := strings.Index(source[loc[0]:], "{") + loc[0]
		body := extractBraceBlock(source, braceStart)
		results = append(results, Service{
			Name: name,
			Body: source[loc[0] : braceStart+len(body)+1],
		})
	}
	return results
}

func extractEnumBlocks(source string) []Enum {
	var results []Enum
	re := regexp.MustCompile(`(?m)^enum\s+(\w+)\s*\{`)
	matches := re.FindAllStringSubmatchIndex(source, -1)
	for _, loc := range matches {
		name := source[loc[2]:loc[3]]
		braceStart := strings.Index(source[loc[0]:], "{") + loc[0]
		body := extractBraceBlock(source, braceStart)
		results = append(results, Enum{
			Name: name,
			Body: source[loc[0] : braceStart+len(body)+1],
		})
	}
	return results
}

// extractBraceBlock returns the content between matched braces starting at pos.
func extractBraceBlock(source string, pos int) string {
	if pos >= len(source) || source[pos] != '{' {
		return ""
	}
	depth := 0
	for i := pos; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[pos+1 : i]
			}
		}
	}
	return source[pos+1:]
}
