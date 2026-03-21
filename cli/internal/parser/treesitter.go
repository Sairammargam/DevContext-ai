package parser

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MultiLangParser parses multiple programming languages
// Uses regex-based parsing as a simpler alternative to tree-sitter
type MultiLangParser struct {
	patterns map[string]LanguagePatterns
}

// LanguagePatterns holds regex patterns for a language
type LanguagePatterns struct {
	Function  *regexp.Regexp
	Class     *regexp.Regexp
	Interface *regexp.Regexp
	Import    *regexp.Regexp
	Method    *regexp.Regexp
	Package   *regexp.Regexp
}

// NewMultiLangParser creates a new multi-language parser
func NewMultiLangParser() *MultiLangParser {
	p := &MultiLangParser{
		patterns: make(map[string]LanguagePatterns),
	}
	p.initPatterns()
	return p
}

func (p *MultiLangParser) initPatterns() {
	// Java patterns
	p.patterns["java"] = LanguagePatterns{
		Package:   regexp.MustCompile(`^package\s+([\w.]+);`),
		Import:    regexp.MustCompile(`^import\s+(?:static\s+)?([\w.*]+);`),
		Class:     regexp.MustCompile(`(?:public|private|protected)?\s*(?:abstract|final)?\s*class\s+(\w+)(?:\s+extends\s+\w+)?(?:\s+implements\s+[\w,\s]+)?\s*\{`),
		Interface: regexp.MustCompile(`(?:public|private|protected)?\s*interface\s+(\w+)(?:\s+extends\s+[\w,\s]+)?\s*\{`),
		Method:    regexp.MustCompile(`(?:public|private|protected)?\s*(?:static|final|abstract|synchronized)?\s*(?:<[\w,\s]+>\s*)?(\w+(?:<[\w,\s<>]+>)?)\s+(\w+)\s*\([^)]*\)`),
		Function:  regexp.MustCompile(`(?:public|private|protected)?\s*(?:static\s+)?[\w<>,\s]+\s+(\w+)\s*\([^)]*\)\s*(?:throws\s+[\w,\s]+)?\s*\{`),
	}

	// Python patterns
	p.patterns["python"] = LanguagePatterns{
		Import:   regexp.MustCompile(`^(?:from\s+([\w.]+)\s+)?import\s+([\w,\s.*]+)`),
		Class:    regexp.MustCompile(`^class\s+(\w+)(?:\([^)]*\))?:`),
		Function: regexp.MustCompile(`^def\s+(\w+)\s*\([^)]*\)(?:\s*->\s*[\w\[\],\s]+)?:`),
		Method:   regexp.MustCompile(`^\s+def\s+(\w+)\s*\(self[^)]*\)(?:\s*->\s*[\w\[\],\s]+)?:`),
	}

	// TypeScript/JavaScript patterns
	p.patterns["typescript"] = LanguagePatterns{
		Import:    regexp.MustCompile(`^import\s+(?:\{[^}]+\}|\*\s+as\s+\w+|\w+)\s+from\s+['"]([^'"]+)['"]`),
		Class:     regexp.MustCompile(`(?:export\s+)?(?:abstract\s+)?class\s+(\w+)(?:\s+extends\s+\w+)?(?:\s+implements\s+[\w,\s]+)?\s*\{`),
		Interface: regexp.MustCompile(`(?:export\s+)?interface\s+(\w+)(?:\s+extends\s+[\w,\s]+)?\s*\{`),
		Function:  regexp.MustCompile(`(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*(?:<[^>]+>)?\s*\([^)]*\)`),
		Method:    regexp.MustCompile(`(?:async\s+)?(\w+)\s*\([^)]*\)\s*(?::\s*[\w<>\[\]|,\s]+)?\s*\{`),
	}

	// JavaScript (same as TypeScript)
	p.patterns["javascript"] = p.patterns["typescript"]

	// Rust patterns
	p.patterns["rust"] = LanguagePatterns{
		Import:    regexp.MustCompile(`^use\s+([\w:]+)(?:::\{[^}]+\})?;`),
		Function:  regexp.MustCompile(`(?:pub\s+)?(?:async\s+)?fn\s+(\w+)(?:<[^>]+>)?\s*\([^)]*\)`),
		Method:    regexp.MustCompile(`^\s+(?:pub\s+)?(?:async\s+)?fn\s+(\w+)(?:<[^>]+>)?\s*\(&?(?:mut\s+)?self[^)]*\)`),
		Class:     regexp.MustCompile(`(?:pub\s+)?struct\s+(\w+)(?:<[^>]+>)?\s*\{?`),
		Interface: regexp.MustCompile(`(?:pub\s+)?trait\s+(\w+)(?:<[^>]+>)?\s*\{`),
	}
}

// ParseFile parses a source file based on its extension
func (p *MultiLangParser) ParseFile(path string) (*ParsedFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lang := detectLanguage(path)
	patterns, ok := p.patterns[lang]
	if !ok {
		// Fallback to basic parsing
		return p.parseBasic(path, string(content), lang)
	}

	return p.parseWithPatterns(path, string(content), lang, patterns)
}

func (p *MultiLangParser) parseWithPatterns(path, content, lang string, patterns LanguagePatterns) (*ParsedFile, error) {
	lines := strings.Split(content, "\n")

	parsed := &ParsedFile{
		Path:      path,
		Language:  lang,
		Content:   content,
		LineCount: len(lines),
	}

	// Extract package (if available)
	if patterns.Package != nil {
		for _, line := range lines[:min(20, len(lines))] {
			if matches := patterns.Package.FindStringSubmatch(line); len(matches) > 1 {
				parsed.Package = matches[1]
				break
			}
		}
	}

	// Extract imports
	if patterns.Import != nil {
		for _, line := range lines {
			if matches := patterns.Import.FindStringSubmatch(line); len(matches) > 1 {
				for _, m := range matches[1:] {
					if m != "" {
						parsed.Imports = append(parsed.Imports, m)
					}
				}
			}
		}
	}

	// Extract symbols
	p.extractSymbols(parsed, lines, patterns)

	return parsed, nil
}

func (p *MultiLangParser) extractSymbols(parsed *ParsedFile, lines []string, patterns LanguagePatterns) {
	var currentClass string
	braceDepth := 0

	for i, line := range lines {
		lineNum := i + 1

		// Track brace depth for context
		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		// Check for class/struct
		if patterns.Class != nil {
			if matches := patterns.Class.FindStringSubmatch(line); len(matches) > 1 {
				currentClass = matches[1]
				sym := Symbol{
					Name:    matches[1],
					Kind:    "class",
					File:    parsed.Path,
					Line:    lineNum,
					EndLine: p.findBlockEnd(lines, i),
					Package: parsed.Package,
				}
				parsed.Symbols = append(parsed.Symbols, sym)
			}
		}

		// Check for interface/trait
		if patterns.Interface != nil {
			if matches := patterns.Interface.FindStringSubmatch(line); len(matches) > 1 {
				sym := Symbol{
					Name:    matches[1],
					Kind:    "interface",
					File:    parsed.Path,
					Line:    lineNum,
					EndLine: p.findBlockEnd(lines, i),
					Package: parsed.Package,
				}
				parsed.Symbols = append(parsed.Symbols, sym)
			}
		}

		// Check for methods (inside a class)
		if patterns.Method != nil && currentClass != "" {
			if matches := patterns.Method.FindStringSubmatch(line); len(matches) > 1 {
				methodName := matches[len(matches)-1]
				if methodName == "" && len(matches) > 1 {
					methodName = matches[1]
				}
				sym := Symbol{
					Name:     methodName,
					Kind:     "method",
					File:     parsed.Path,
					Line:     lineNum,
					EndLine:  p.findBlockEnd(lines, i),
					Package:  parsed.Package,
					Receiver: currentClass,
				}
				parsed.Symbols = append(parsed.Symbols, sym)
			}
		}

		// Check for standalone functions
		if patterns.Function != nil {
			if matches := patterns.Function.FindStringSubmatch(line); len(matches) > 1 {
				// Skip if this looks like a method inside a class
				if currentClass != "" && braceDepth > 1 {
					continue
				}
				sym := Symbol{
					Name:    matches[1],
					Kind:    "function",
					File:    parsed.Path,
					Line:    lineNum,
					EndLine: p.findBlockEnd(lines, i),
					Package: parsed.Package,
				}
				parsed.Symbols = append(parsed.Symbols, sym)
			}
		}

		// Reset class context when exiting
		if braceDepth == 0 {
			currentClass = ""
		}
	}
}

func (p *MultiLangParser) findBlockEnd(lines []string, startIdx int) int {
	depth := 0
	started := false

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		opens := strings.Count(line, "{")
		closes := strings.Count(line, "}")

		// For Python, use indentation
		if opens == 0 && closes == 0 && i > startIdx {
			// Check if we're in a Python-like language
			if strings.HasSuffix(strings.TrimSpace(lines[startIdx]), ":") {
				// Indentation-based block end
				startIndent := len(lines[startIdx]) - len(strings.TrimLeft(lines[startIdx], " \t"))
				currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))
				if strings.TrimSpace(line) != "" && currentIndent <= startIndent {
					return i
				}
				continue
			}
		}

		depth += opens - closes
		if opens > 0 {
			started = true
		}

		if started && depth <= 0 {
			return i + 1
		}
	}

	return len(lines)
}

func (p *MultiLangParser) parseBasic(path, content, lang string) (*ParsedFile, error) {
	lines := strings.Split(content, "\n")

	return &ParsedFile{
		Path:      path,
		Language:  lang,
		Content:   content,
		LineCount: len(lines),
	}, nil
}

// ParseDirectory parses all supported files in a directory
func (p *MultiLangParser) ParseDirectory(dir string) ([]*ParsedFile, error) {
	var files []*ParsedFile

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && shouldSkipDir(info.Name()) {
			return filepath.SkipDir
		}

		if !info.IsDir() && isSupportedFile(path) {
			parsed, err := p.ParseFile(path)
			if err != nil {
				return nil // Skip files that fail to parse
			}
			files = append(files, parsed)
		}

		return nil
	})

	return files, err
}

func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".java":
		return "java"
	case ".py":
		return "python"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	default:
		return "unknown"
	}
}

func isSupportedFile(path string) bool {
	supported := map[string]bool{
		".go": true, ".java": true, ".py": true,
		".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".rs": true, ".rb": true, ".php": true,
		".c": true, ".h": true, ".cpp": true, ".cc": true,
		".cs": true, ".swift": true, ".kt": true, ".scala": true,
	}
	ext := strings.ToLower(filepath.Ext(path))
	return supported[ext]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
