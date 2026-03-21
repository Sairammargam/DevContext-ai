package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Symbol represents a parsed code symbol
type Symbol struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"` // package, type, function, method, variable, import
	File       string   `json:"file"`
	Line       int      `json:"line"`
	EndLine    int      `json:"endLine"`
	Signature  string   `json:"signature,omitempty"`
	Doc        string   `json:"doc,omitempty"`
	Package    string   `json:"package,omitempty"`
	Receiver   string   `json:"receiver,omitempty"` // For methods
	Imports    []string `json:"imports,omitempty"`
	Calls      []string `json:"calls,omitempty"`      // Functions/methods called
	References []string `json:"references,omitempty"` // Types/vars referenced
}

// ParsedFile represents a parsed source file
type ParsedFile struct {
	Path      string   `json:"path"`
	Language  string   `json:"language"`
	Package   string   `json:"package,omitempty"`
	Imports   []string `json:"imports,omitempty"`
	Symbols   []Symbol `json:"symbols"`
	Content   string   `json:"content"`
	LineCount int      `json:"lineCount"`
}

// GoParser parses Go source files
type GoParser struct {
	fset *token.FileSet
}

// NewGoParser creates a new Go parser
func NewGoParser() *GoParser {
	return &GoParser{
		fset: token.NewFileSet(),
	}
}

// ParseFile parses a single Go file
func (p *GoParser) ParseFile(path string) (*ParsedFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	f, err := parser.ParseFile(p.fset, path, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}

	parsed := &ParsedFile{
		Path:      path,
		Language:  "go",
		Package:   f.Name.Name,
		Content:   string(content),
		LineCount: strings.Count(string(content), "\n") + 1,
	}

	// Extract imports
	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		parsed.Imports = append(parsed.Imports, importPath)
	}

	// Extract symbols
	ast.Inspect(f, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			sym := p.parseFuncDecl(decl, path, f.Name.Name)
			parsed.Symbols = append(parsed.Symbols, sym)

		case *ast.GenDecl:
			symbols := p.parseGenDecl(decl, path, f.Name.Name)
			parsed.Symbols = append(parsed.Symbols, symbols...)
		}
		return true
	})

	return parsed, nil
}

// ParseDirectory parses all Go files in a directory
func (p *GoParser) ParseDirectory(dir string) ([]*ParsedFile, error) {
	var files []*ParsedFile

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor, node_modules, .git, etc.
		if info.IsDir() && shouldSkipDir(info.Name()) {
			return filepath.SkipDir
		}

		// Only parse .go files
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			parsed, err := p.ParseFile(path)
			if err != nil {
				// Log but don't fail on individual file errors
				fmt.Printf("Warning: failed to parse %s: %v\n", path, err)
				return nil
			}
			files = append(files, parsed)
		}

		return nil
	})

	return files, err
}

func (p *GoParser) parseFuncDecl(decl *ast.FuncDecl, path, pkg string) Symbol {
	sym := Symbol{
		Name:    decl.Name.Name,
		Kind:    "function",
		File:    path,
		Line:    p.fset.Position(decl.Pos()).Line,
		EndLine: p.fset.Position(decl.End()).Line,
		Package: pkg,
	}

	// Check if it's a method
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		sym.Kind = "method"
		if t, ok := decl.Recv.List[0].Type.(*ast.StarExpr); ok {
			if ident, ok := t.X.(*ast.Ident); ok {
				sym.Receiver = ident.Name
			}
		} else if ident, ok := decl.Recv.List[0].Type.(*ast.Ident); ok {
			sym.Receiver = ident.Name
		}
	}

	// Build signature
	sym.Signature = p.buildFuncSignature(decl)

	// Extract doc comment
	if decl.Doc != nil {
		sym.Doc = decl.Doc.Text()
	}

	// Extract function calls
	sym.Calls = p.extractCalls(decl.Body)

	return sym
}

func (p *GoParser) parseGenDecl(decl *ast.GenDecl, path, pkg string) []Symbol {
	var symbols []Symbol

	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			sym := Symbol{
				Name:    s.Name.Name,
				Kind:    "type",
				File:    path,
				Line:    p.fset.Position(s.Pos()).Line,
				EndLine: p.fset.Position(s.End()).Line,
				Package: pkg,
			}

			// Determine type kind (struct, interface, etc.)
			switch s.Type.(type) {
			case *ast.StructType:
				sym.Kind = "struct"
			case *ast.InterfaceType:
				sym.Kind = "interface"
			}

			if decl.Doc != nil {
				sym.Doc = decl.Doc.Text()
			}

			symbols = append(symbols, sym)

		case *ast.ValueSpec:
			for _, name := range s.Names {
				kind := "variable"
				if decl.Tok == token.CONST {
					kind = "const"
				}

				sym := Symbol{
					Name:    name.Name,
					Kind:    kind,
					File:    path,
					Line:    p.fset.Position(s.Pos()).Line,
					EndLine: p.fset.Position(s.End()).Line,
					Package: pkg,
				}

				if decl.Doc != nil {
					sym.Doc = decl.Doc.Text()
				}

				symbols = append(symbols, sym)
			}
		}
	}

	return symbols
}

func (p *GoParser) buildFuncSignature(decl *ast.FuncDecl) string {
	var sig strings.Builder

	sig.WriteString("func ")

	// Receiver for methods
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		sig.WriteString("(")
		for i, field := range decl.Recv.List {
			if i > 0 {
				sig.WriteString(", ")
			}
			sig.WriteString(p.typeToString(field.Type))
		}
		sig.WriteString(") ")
	}

	sig.WriteString(decl.Name.Name)
	sig.WriteString("(")

	// Parameters
	if decl.Type.Params != nil {
		for i, field := range decl.Type.Params.List {
			if i > 0 {
				sig.WriteString(", ")
			}
			for j, name := range field.Names {
				if j > 0 {
					sig.WriteString(", ")
				}
				sig.WriteString(name.Name)
			}
			if len(field.Names) > 0 {
				sig.WriteString(" ")
			}
			sig.WriteString(p.typeToString(field.Type))
		}
	}

	sig.WriteString(")")

	// Return types
	if decl.Type.Results != nil && len(decl.Type.Results.List) > 0 {
		sig.WriteString(" ")
		if len(decl.Type.Results.List) > 1 {
			sig.WriteString("(")
		}
		for i, field := range decl.Type.Results.List {
			if i > 0 {
				sig.WriteString(", ")
			}
			sig.WriteString(p.typeToString(field.Type))
		}
		if len(decl.Type.Results.List) > 1 {
			sig.WriteString(")")
		}
	}

	return sig.String()
}

func (p *GoParser) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + p.typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + p.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + p.typeToString(t.Key) + "]" + p.typeToString(t.Value)
	case *ast.SelectorExpr:
		return p.typeToString(t.X) + "." + t.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + p.typeToString(t.Value)
	case *ast.Ellipsis:
		return "..." + p.typeToString(t.Elt)
	default:
		return "unknown"
	}
}

func (p *GoParser) extractCalls(body *ast.BlockStmt) []string {
	if body == nil {
		return nil
	}

	var calls []string
	seen := make(map[string]bool)

	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			var name string
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				name = fn.Name
			case *ast.SelectorExpr:
				if ident, ok := fn.X.(*ast.Ident); ok {
					name = ident.Name + "." + fn.Sel.Name
				}
			}
			if name != "" && !seen[name] {
				seen[name] = true
				calls = append(calls, name)
			}
		}
		return true
	})

	return calls
}

func shouldSkipDir(name string) bool {
	skipDirs := map[string]bool{
		"vendor":       true,
		"node_modules": true,
		".git":         true,
		".svn":         true,
		"__pycache__":  true,
		".idea":        true,
		".vscode":      true,
		"target":       true,
		"build":        true,
		"dist":         true,
		"bin":          true,
	}
	return skipDirs[name]
}
