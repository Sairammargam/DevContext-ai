package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Styles for terminal output
var (
	// Agent label styles
	AgentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			PaddingRight(1)

	// User prompt style
	UserStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("14"))

	// Code block style
	CodeStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	// File path style
	FileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Italic(true)

	// Error style
	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("9"))

	// Success style
	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("10"))

	// Mermaid diagram style
	MermaidStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1).
			Margin(1, 0)

	// Citation style
	CitationStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
)

// Printer handles streaming output to terminal
type Printer struct {
	agentName string
	buffer    strings.Builder
	inCode    bool
	codeLang  string
}

// NewPrinter creates a new printer for an agent
func NewPrinter(agentName string) *Printer {
	return &Printer{agentName: agentName}
}

// PrintAgentLabel prints the agent label
func (p *Printer) PrintAgentLabel() {
	fmt.Print(AgentStyle.Render(p.agentName + ":"))
	fmt.Print(" ")
}

// PrintToken prints a single token with streaming effect
func (p *Printer) PrintToken(token string) {
	// Check for code block markers
	if strings.Contains(token, "```") {
		p.handleCodeBlock(token)
		return
	}

	// Handle mermaid diagrams
	if p.inCode && (p.codeLang == "mermaid" || p.codeLang == "graph") {
		p.buffer.WriteString(token)
		return
	}

	// Regular output
	if p.inCode {
		fmt.Print(CodeStyle.Render(token))
	} else {
		fmt.Print(token)
	}
}

func (p *Printer) handleCodeBlock(token string) {
	parts := strings.Split(token, "```")
	for i, part := range parts {
		if i > 0 {
			// Toggle code block state
			p.inCode = !p.inCode
			if p.inCode {
				// Starting code block - detect language
				p.codeLang = strings.TrimSpace(part)
				p.buffer.Reset()
				fmt.Println()
			} else {
				// Ending code block
				if p.codeLang == "mermaid" || p.codeLang == "graph" {
					fmt.Println(MermaidStyle.Render(p.buffer.String()))
				}
				fmt.Println()
				p.codeLang = ""
			}
		} else if part != "" {
			if p.inCode {
				p.buffer.WriteString(part)
			} else {
				fmt.Print(part)
			}
		}
	}
}

// PrintNewLine prints a newline
func (p *Printer) PrintNewLine() {
	fmt.Println()
}

// PrintUserPrompt prints the user prompt prefix
func PrintUserPrompt() {
	fmt.Print(UserStyle.Render("? "))
}

// PrintError prints an error message
func PrintError(msg string) {
	fmt.Println(ErrorStyle.Render("Error: " + msg))
}

// PrintSuccess prints a success message
func PrintSuccess(msg string) {
	fmt.Println(SuccessStyle.Render("Success: " + msg))
}

// PrintFileCitation prints a file citation
func PrintFileCitation(files []string) {
	if len(files) == 0 {
		return
	}
	fmt.Println()
	fmt.Print(CitationStyle.Render("Sources: "))
	for i, f := range files {
		if i > 0 {
			fmt.Print(CitationStyle.Render(", "))
		}
		fmt.Print(FileStyle.Render(f))
	}
	fmt.Println()
}

// FormatMermaidDiagram formats a mermaid diagram for display
func FormatMermaidDiagram(diagram string) string {
	return MermaidStyle.Render(diagram)
}

// Box creates a boxed message
func Box(title, content string) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Margin(1, 0)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("63"))

	return boxStyle.Render(titleStyle.Render(title) + "\n\n" + content)
}
