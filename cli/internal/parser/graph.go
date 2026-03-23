package parser

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/marcboeker/go-duckdb"
)

// GraphNode represents a node in the dependency graph
type GraphNode struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"` // service, file, symbol, package
	File    string `json:"file"`
	Package string `json:"package"`
}

// GraphEdge represents an edge in the dependency graph
type GraphEdge struct {
	ID       string `json:"id"`
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Kind     string `json:"kind"` // calls, imports, extends, implements
}

// DependencyGraph manages the code dependency graph using DuckDB
type DependencyGraph struct {
	db   *sql.DB
	path string
}

// NewDependencyGraph creates or opens a dependency graph database
func NewDependencyGraph(dbPath string) (*DependencyGraph, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open DuckDB: %w", err)
	}

	g := &DependencyGraph{db: db, path: dbPath}

	if err := g.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return g, nil
}

func (g *DependencyGraph) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS nodes (
			id VARCHAR PRIMARY KEY,
			name VARCHAR NOT NULL,
			kind VARCHAR NOT NULL,
			file VARCHAR,
			package VARCHAR,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS edges (
			id VARCHAR PRIMARY KEY,
			source_id VARCHAR NOT NULL,
			target_id VARCHAR NOT NULL,
			kind VARCHAR NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (source_id) REFERENCES nodes(id),
			FOREIGN KEY (target_id) REFERENCES nodes(id)
		);

		CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name);
		CREATE INDEX IF NOT EXISTS idx_nodes_kind ON nodes(kind);
		CREATE INDEX IF NOT EXISTS idx_nodes_file ON nodes(file);
		CREATE INDEX IF NOT EXISTS idx_edges_source ON edges(source_id);
		CREATE INDEX IF NOT EXISTS idx_edges_target ON edges(target_id);
		CREATE INDEX IF NOT EXISTS idx_edges_kind ON edges(kind);
	`

	_, err := g.db.Exec(schema)
	return err
}

// Clear removes all data from the graph
func (g *DependencyGraph) Clear() error {
	_, err := g.db.Exec("DELETE FROM edges; DELETE FROM nodes;")
	return err
}

// AddNode adds a node to the graph
func (g *DependencyGraph) AddNode(node GraphNode) error {
	_, err := g.db.Exec(`
		INSERT OR REPLACE INTO nodes (id, name, kind, file, package)
		VALUES (?, ?, ?, ?, ?)
	`, node.ID, node.Name, node.Kind, node.File, node.Package)
	return err
}

// AddEdge adds an edge to the graph
func (g *DependencyGraph) AddEdge(edge GraphEdge) error {
	_, err := g.db.Exec(`
		INSERT OR REPLACE INTO edges (id, source_id, target_id, kind)
		VALUES (?, ?, ?, ?)
	`, edge.ID, edge.SourceID, edge.TargetID, edge.Kind)
	return err
}

// BuildFromParsedFiles builds the graph from parsed source files
func (g *DependencyGraph) BuildFromParsedFiles(files []*ParsedFile) error {
	// First pass: Create nodes for all symbols
	nodeMap := make(map[string]string) // symbol name -> node ID

	for _, file := range files {
		// Add file node
		fileNodeID := fmt.Sprintf("file:%s", file.Path)
		if err := g.AddNode(GraphNode{
			ID:      fileNodeID,
			Name:    filepath.Base(file.Path),
			Kind:    "file",
			File:    file.Path,
			Package: file.Package,
		}); err != nil {
			return err
		}

		// Add package node if present
		if file.Package != "" {
			pkgNodeID := fmt.Sprintf("pkg:%s", file.Package)
			if err := g.AddNode(GraphNode{
				ID:      pkgNodeID,
				Name:    file.Package,
				Kind:    "package",
				Package: file.Package,
			}); err != nil {
				return err
			}

			// File belongs to package
			if err := g.AddEdge(GraphEdge{
				ID:       fmt.Sprintf("%s->%s", fileNodeID, pkgNodeID),
				SourceID: fileNodeID,
				TargetID: pkgNodeID,
				Kind:     "belongs_to",
			}); err != nil {
				return err
			}
		}

		// Add symbol nodes
		for _, sym := range file.Symbols {
			nodeID := fmt.Sprintf("sym:%s:%s:%d", file.Path, sym.Name, sym.Line)
			fullName := sym.Name
			if sym.Receiver != "" {
				fullName = sym.Receiver + "." + sym.Name
			}
			if file.Package != "" {
				fullName = file.Package + "." + fullName
			}

			if err := g.AddNode(GraphNode{
				ID:      nodeID,
				Name:    sym.Name,
				Kind:    sym.Kind,
				File:    file.Path,
				Package: file.Package,
			}); err != nil {
				return err
			}

			nodeMap[fullName] = nodeID
			nodeMap[sym.Name] = nodeID // Also map short name

			// Symbol belongs to file
			if err := g.AddEdge(GraphEdge{
				ID:       fmt.Sprintf("%s->%s", nodeID, fileNodeID),
				SourceID: nodeID,
				TargetID: fileNodeID,
				Kind:     "defined_in",
			}); err != nil {
				return err
			}
		}
	}

	// Second pass: Create edges for calls and imports
	for _, file := range files {
		fileNodeID := fmt.Sprintf("file:%s", file.Path)

		// Import edges
		for _, imp := range file.Imports {
			impNodeID := fmt.Sprintf("pkg:%s", imp)
			// Create import package node if it doesn't exist
			g.AddNode(GraphNode{
				ID:      impNodeID,
				Name:    imp,
				Kind:    "package",
				Package: imp,
			})

			if err := g.AddEdge(GraphEdge{
				ID:       fmt.Sprintf("%s-imports->%s", fileNodeID, impNodeID),
				SourceID: fileNodeID,
				TargetID: impNodeID,
				Kind:     "imports",
			}); err != nil {
				return err
			}
		}

		// Call edges
		for _, sym := range file.Symbols {
			sourceID := fmt.Sprintf("sym:%s:%s:%d", file.Path, sym.Name, sym.Line)

			for _, call := range sym.Calls {
				// Try to find the target node
				if targetID, ok := nodeMap[call]; ok {
					if err := g.AddEdge(GraphEdge{
						ID:       fmt.Sprintf("%s-calls->%s", sourceID, targetID),
						SourceID: sourceID,
						TargetID: targetID,
						Kind:     "calls",
					}); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// FindPath finds the shortest path between two nodes
func (g *DependencyGraph) FindPath(sourceName, targetName string) ([]GraphNode, error) {
	// Use recursive CTE to find path
	query := `
		WITH RECURSIVE path AS (
			SELECT
				source_id,
				target_id,
				1 as depth,
				source_id || '->' || target_id as path_str
			FROM edges
			WHERE source_id IN (SELECT id FROM nodes WHERE name LIKE ?)

			UNION ALL

			SELECT
				p.source_id,
				e.target_id,
				p.depth + 1,
				p.path_str || '->' || e.target_id
			FROM path p
			JOIN edges e ON p.target_id = e.source_id
			WHERE p.depth < 10
			AND p.path_str NOT LIKE '%' || e.target_id || '%'
		)
		SELECT DISTINCT path_str
		FROM path
		WHERE target_id IN (SELECT id FROM nodes WHERE name LIKE ?)
		ORDER BY depth
		LIMIT 1
	`

	row := g.db.QueryRow(query, "%"+sourceName+"%", "%"+targetName+"%")

	var pathStr string
	if err := row.Scan(&pathStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No path found
		}
		return nil, err
	}

	// Parse path and get nodes
	// (simplified - would need to parse pathStr and look up each node)
	return nil, nil
}

// GetCallers returns all functions that call the given function
func (g *DependencyGraph) GetCallers(symbolName string) ([]GraphNode, error) {
	query := `
		SELECT n.id, n.name, n.kind, n.file, n.package
		FROM nodes n
		JOIN edges e ON n.id = e.source_id
		WHERE e.kind = 'calls'
		AND e.target_id IN (SELECT id FROM nodes WHERE name = ?)
	`

	rows, err := g.db.Query(query, symbolName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []GraphNode
	for rows.Next() {
		var n GraphNode
		if err := rows.Scan(&n.ID, &n.Name, &n.Kind, &n.File, &n.Package); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return nodes, nil
}

// GetCallees returns all functions called by the given function
func (g *DependencyGraph) GetCallees(symbolName string) ([]GraphNode, error) {
	query := `
		SELECT n.id, n.name, n.kind, n.file, n.package
		FROM nodes n
		JOIN edges e ON n.id = e.target_id
		WHERE e.kind = 'calls'
		AND e.source_id IN (SELECT id FROM nodes WHERE name = ?)
	`

	rows, err := g.db.Query(query, symbolName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []GraphNode
	for rows.Next() {
		var n GraphNode
		if err := rows.Scan(&n.ID, &n.Name, &n.Kind, &n.File, &n.Package); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return nodes, nil
}

// GetStatistics returns graph statistics
func (g *DependencyGraph) GetStatistics() (map[string]int, error) {
	stats := make(map[string]int)

	// Count nodes by kind
	rows, err := g.db.Query("SELECT kind, COUNT(*) FROM nodes GROUP BY kind")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			return nil, err
		}
		stats["nodes_"+kind] = count
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Count edges by kind
	rows2, err := g.db.Query("SELECT kind, COUNT(*) FROM edges GROUP BY kind")
	if err != nil {
		return nil, err
	}
	defer rows2.Close()

	for rows2.Next() {
		var kind string
		var count int
		if err := rows2.Scan(&kind, &count); err != nil {
			return nil, err
		}
		stats["edges_"+kind] = count
	}

	if err := rows2.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

// Close closes the database connection
func (g *DependencyGraph) Close() error {
	return g.db.Close()
}
