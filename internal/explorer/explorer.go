package explorer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"atlas.explorer/internal/filesystem"
	"atlas.explorer/internal/ui"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

type Model struct {
	path          string
	files         []filesystem.FileInfo
	cursor        int
	top           int // Top of the viewport
	selected      map[string]bool
	clipboard     []string
	isCut         bool
	width, height int
	message       string
	err           error
	sorting       string // "name", "size", "time"

	// Input and state
	input      textinput.Model
	isInput    bool
	isConfirm  bool
	toDelete   []string
}

func NewModel() Model {
	cwd, _ := os.Getwd()
	ti := textinput.New()
	ti.Placeholder = "Enter path..."
	ti.Focus()
	
	m := Model{
		path:     cwd,
		selected: make(map[string]bool),
		sorting:  "name",
		input:    ti,
		height:   20, // Default height before WindowSizeMsg
	}
	m.loadFiles()
	return m
}

func (m *Model) loadFiles() {
	m.err = nil
	files, err := filesystem.ListDir(m.path)
	
	parent := filepath.Dir(m.path)
	hasParent := parent != m.path

	if err != nil {
		m.err = err
		m.files = nil
		if hasParent {
			m.files = []filesystem.FileInfo{{
				Name:  "..",
				Path:  parent,
				IsDir: true,
			}}
		}
		m.cursor = 0
		m.top = 0
		return
	}
	
	m.files = files
	m.sortFiles()

	// Add ".." entry at the top if not at root
	if hasParent {
		m.files = append([]filesystem.FileInfo{{
			Name:  "..",
			Path:  parent,
			IsDir: true,
		}}, m.files...)
	}

	if m.cursor >= len(m.files) {
		m.cursor = max(0, len(m.files)-1)
	}
	m.updateViewport()
}

func (m *Model) updateViewport() {
	headerHeight := 6 // Path + Header + Info
	if m.isInput { headerHeight += 2 }
	if m.isConfirm { headerHeight += 2 }
	footerHeight := 3
	
	visibleHeight := m.height - headerHeight - footerHeight
	if visibleHeight <= 0 { visibleHeight = 10 }

	if m.cursor < m.top {
		m.top = m.cursor
	} else if m.cursor >= m.top+visibleHeight {
		m.top = m.cursor - visibleHeight + 1
	}
}

func (m *Model) sortFiles() {
	sort.Slice(m.files, func(i, j int) bool {
		// Dirs first (except ".." which is handled manually)
		if m.files[i].IsDir && !m.files[j].IsDir {
			return true
		}
		if !m.files[i].IsDir && m.files[j].IsDir {
			return false
		}

		switch m.sorting {
		case "size":
			return m.files[i].Size > m.files[j].Size
		case "time":
			return m.files[i].ModTime.After(m.files[j].ModTime)
		default:
			return strings.ToLower(m.files[i].Name) < strings.ToLower(m.files[j].Name)
		}
	})
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tickMsg:
		m.message = ""
		return m, nil

	case tea.KeyMsg:
		if m.isInput {
			switch msg.String() {
			case "enter":
				m.isInput = false
				newPath := m.input.Value()
				if _, err := os.Stat(newPath); err == nil {
					m.path = newPath
					m.cursor = 0
					m.top = 0
					m.loadFiles()
				} else {
					m.message = "Invalid path: " + newPath
					return m, tick()
				}
				m.input.SetValue("")
			case "esc":
				m.isInput = false
				m.input.SetValue("")
			default:
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		if m.isConfirm {
			switch msg.String() {
			case "y", "Y":
				for _, p := range m.toDelete {
					filesystem.Delete(p)
				}
				m.isConfirm = false
				m.toDelete = nil
				m.loadFiles()
				m.message = "Deleted items"
				return m, tick()
			case "n", "N", "esc":
				m.isConfirm = false
				m.toDelete = nil
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "j", "down":
			if m.cursor < len(m.files)-1 {
				m.cursor++
				m.updateViewport()
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.updateViewport()
			}

		case "h", "left", "backspace":
			parent := filepath.Dir(m.path)
			if parent != m.path {
				m.path = parent
				m.cursor = 0
				m.top = 0
				m.loadFiles()
			}

		case "l", "right", "enter":
			if len(m.files) > 0 {
				f := m.files[m.cursor]
				if f.IsDir {
					m.path = f.Path
					m.cursor = 0
					m.top = 0
					m.loadFiles()
				} else {
					if f.Name != ".." {
						filesystem.OpenFile(f.Path)
						m.message = fmt.Sprintf("Opened %s", f.Name)
						return m, tick()
					}
				}
			}

		case "/": // Go to path
			m.isInput = true
			m.input.SetValue(m.path)
			m.input.Focus()
			m.updateViewport()
			return m, nil

		case " ":
			if len(m.files) > 0 {
				f := m.files[m.cursor]
				if f.Name == ".." {
					break
				}
				if m.selected[f.Path] {
					delete(m.selected, f.Path)
				} else {
					m.selected[f.Path] = true
				}
			}

		case "c": // Copy
			m.clipboard = []string{}
			if len(m.selected) > 0 {
				for p := range m.selected {
					m.clipboard = append(m.clipboard, p)
				}
			} else if len(m.files) > 0 {
				if m.files[m.cursor].Name != ".." {
					m.clipboard = append(m.clipboard, m.files[m.cursor].Path)
				}
			}
			m.isCut = false
			m.message = fmt.Sprintf("Copied %d items", len(m.clipboard))
			return m, tick()

		case "x": // Cut
			m.clipboard = []string{}
			if len(m.selected) > 0 {
				for p := range m.selected {
					m.clipboard = append(m.clipboard, p)
				}
			} else if len(m.files) > 0 {
				if m.files[m.cursor].Name != ".." {
					m.clipboard = append(m.clipboard, m.files[m.cursor].Path)
				}
			}
			m.isCut = true
			m.message = fmt.Sprintf("Cut %d items", len(m.clipboard))
			return m, tick()

		case "p": // Paste
			if len(m.clipboard) > 0 {
				for _, src := range m.clipboard {
					dst := filepath.Join(m.path, filepath.Base(src))
					if m.isCut {
						filesystem.Move(src, dst)
					} else {
						filesystem.Copy(src, dst)
					}
				}
				m.loadFiles()
				m.message = "Pasted items"
				if m.isCut {
					m.clipboard = nil
				}
				return m, tick()
			}

		case "d": // Delete
			m.toDelete = nil
			if len(m.selected) > 0 {
				for p := range m.selected {
					m.toDelete = append(m.toDelete, p)
				}
				m.selected = make(map[string]bool)
			} else if len(m.files) > 0 {
				if m.files[m.cursor].Name != ".." {
					m.toDelete = append(m.toDelete, m.files[m.cursor].Path)
				}
			}
			if len(m.toDelete) > 0 {
				m.isConfirm = true
				m.updateViewport()
			}

		case "s": // Sort
			switch m.sorting {
			case "name": m.sorting = "size"
			case "size": m.sorting = "time"
			default: m.sorting = "name"
			}
			m.sortFiles()
			m.message = "Sorted by " + m.sorting
			return m, tick()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewport()
	}

	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	// Header
	header := ui.HeaderStyle.Render("Atlas Explorer")
	path := ui.PathStyle.Render(m.path)
	
	var infoText string
	if m.err != nil {
		infoText = ui.WarningStyle.Render(fmt.Sprintf("⚠️ Error: %v", m.err))
	} else {
		infoText = ui.InfoStyle.Render(fmt.Sprintf("%d items • Sorted by %s", len(m.files), m.sorting))
	}
	
	s.WriteString(lipgloss.JoinVertical(lipgloss.Left, header, path, infoText, ""))
	s.WriteString("\n")

	// Input view
	if m.isInput {
		s.WriteString("Go to: " + m.input.View() + "\n\n")
	}

	// Confirm view
	if m.isConfirm {
		s.WriteString(ui.WarningStyle.Render(fmt.Sprintf("Delete %d items? (y/n)", len(m.toDelete))) + "\n\n")
	}

	// Calculate visible range
	headerHeight := 6
	if m.isInput { headerHeight += 2 }
	if m.isConfirm { headerHeight += 2 }
	footerHeight := 3
	visibleHeight := m.height - headerHeight - footerHeight
	if visibleHeight <= 0 { visibleHeight = 10 }

	end := m.top + visibleHeight
	if end > len(m.files) {
		end = len(m.files)
	}

	// File List
	if len(m.files) == 0 && m.err == nil {
		s.WriteString("\n  (Empty directory)\n")
	}

	for i := m.top; i < end; i++ {
		f := m.files[i]
		
		// Indicator and Selection
		indicator := "  "
		if m.cursor == i {
			indicator = "» "
		}

		selected := " "
		if m.selected[f.Path] {
			selected = "●"
		}
		if m.selected[f.Path] {
			selected = ui.SelectedStyle.Render(selected)
		}

		icon := ui.GetIcon(f.IsDir, f.Name)
		name := f.Name
		
		// Info
		size := ""
		if !f.IsDir && f.Name != ".." {
			size = fmt.Sprintf("%d bytes", f.Size)
		}
		
		// Styling
		var nameStr string
		if f.IsDir {
			nameStr = ui.DirStyle.Render(name)
		} else {
			nameStr = ui.FileStyle.Render(name)
		}
		
		infoStr := ui.InfoStyle.Render(size)

		line := fmt.Sprintf("%s %s %s %-40s %s", indicator, selected, icon, nameStr, infoStr)
		
		// Full line highlight
		if m.cursor == i {
			// We need to calculate the actual string length to pad it for full line background
			// Lipgloss strings contain ANSI codes, so we use lipgloss.Width
			lineWidth := lipgloss.Width(line)
			padding := m.width - lineWidth
			if padding < 0 { padding = 0 }
			line += strings.Repeat(" ", padding)
			s.WriteString(ui.CursorStyle.Render(line))
		} else {
			s.WriteString(line)
		}
		s.WriteString("\n")
	}

	// Fill remaining space
	remaining := visibleHeight - (end - m.top)
	for i := 0; i < remaining; i++ {
		s.WriteString("\n")
	}

	// Footer
	s.WriteString("\n")
	if m.message != "" {
		s.WriteString(ui.SuccessStyle.Render(" LOG: " + m.message))
	} else {
		s.WriteString(ui.InfoStyle.Render("h/j/k/l: Navigate • /: Go to • enter: Open • space: Select • c/x/p: Copy/Cut/Paste • d: Delete • s: Sort • q: Quit"))
	}

	return s.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
