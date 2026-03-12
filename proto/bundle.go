package proto

import (
	"fmt"
	"sort"
	"strings"
)

// Bundle merges multiple parsed proto files into a single proto source string.
// It deduplicates messages, services, and enums by name.
func Bundle(files []*File) string {
	var b strings.Builder

	b.WriteString("// Auto-generated bundled proto file.\n")
	b.WriteString(`syntax = "proto3";` + "\n\n")
	b.WriteString("package bundle;\n\n")

	// Collect unique imports
	importSet := map[string]bool{}
	for _, f := range files {
		for _, imp := range f.Imports {
			importSet[imp] = true
		}
	}
	imports := sortedKeys(importSet)
	for _, imp := range imports {
		b.WriteString(fmt.Sprintf("import \"%s\";\n", imp))
	}
	if len(imports) > 0 {
		b.WriteString("\n")
	}

	// Collect unique enums
	enumSeen := map[string]bool{}
	for _, f := range files {
		for _, e := range f.Enums {
			if !enumSeen[e.Name] {
				enumSeen[e.Name] = true
				b.WriteString(e.Body + "\n\n")
			}
		}
	}

	// Collect unique messages
	msgSeen := map[string]bool{}
	for _, f := range files {
		for _, m := range f.Messages {
			if !msgSeen[m.Name] {
				msgSeen[m.Name] = true
				b.WriteString(m.Body + "\n\n")
			}
		}
	}

	// Collect unique services
	svcSeen := map[string]bool{}
	for _, f := range files {
		for _, s := range f.Services {
			if !svcSeen[s.Name] {
				svcSeen[s.Name] = true
				b.WriteString(s.Body + "\n\n")
			}
		}
	}

	return b.String()
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
