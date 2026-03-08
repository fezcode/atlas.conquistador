package ui

import "github.com/charmbracelet/lipgloss"

var (
	SelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("201")).Bold(true)
	HeaderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Padding(0, 1)
	DirStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	FileStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	PathStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	InfoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	WarningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	SuccessStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	
	CursorStyle = lipgloss.NewStyle().Background(lipgloss.Color("57")).Foreground(lipgloss.Color("255")).Bold(true)
	InactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Box Styles
	MainBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1)

	HeaderBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("238"))

	FooterBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("238"))

	LineNumberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	DividerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	EOFStyle        = lipgloss.NewStyle().
			Background(lipgloss.Color("160")).
			Foreground(lipgloss.Color("255")).
			Bold(true).
			Padding(0, 1)
)

func GetIcon(isDir bool, name string) string {
	if isDir {
		return "📁"
	}
	// Simple extension based icons
	ext := ""
	if idx := lastIndex(name, "."); idx != -1 {
		ext = name[idx:]
	}
	switch ext {
	case ".go", ".py", ".js", ".ts", ".c", ".cpp": return "💻"
	case ".md", ".txt", ".piml": return "📝"
	case ".jpg", ".png", ".gif", ".svg": return "🖼️"
	case ".mp3", ".wav", ".flac": return "🎵"
	case ".mp4", ".mkv", ".mov": return "🎬"
	case ".zip", ".rar", ".7z", ".tar", ".gz": return "📦"
	case ".exe", ".sh", ".bat", ".ps1": return "🚀"
	default: return "📄"
	}
}

func lastIndex(s, sep string) int {
	for i := len(s) - len(sep); i >= 0; i-- {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}
