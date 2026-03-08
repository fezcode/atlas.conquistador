package ui

import "github.com/charmbracelet/lipgloss"

var (
	SelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("201")).Bold(true)
	HeaderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Padding(0, 1)
	DirStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	FileStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	PathStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	InfoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	WarningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	SuccessStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	
	CursorStyle = lipgloss.NewStyle().Background(lipgloss.Color("57")).Foreground(lipgloss.Color("255")).Bold(true)
	InactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
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
