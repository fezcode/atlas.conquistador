package explorer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"atlas.conquistador/internal/filesystem"
	"atlas.conquistador/internal/ui"
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
	isHelp     bool
	isViewer   bool
	toDelete   []string

	// Viewer state
	viewerContent []string
	viewerPath    string
	viewerTop     int
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
	m.loadFiles("")
	return m
}

func (m *Model) loadFiles(targetName string) {
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

	// Highlight target if provided
	if targetName != "" {
		for i, f := range m.files {
			if f.Name == targetName {
				m.cursor = i
				break
			}
		}
	} else if m.cursor >= len(m.files) {
		m.cursor = max(0, len(m.files)-1)
	}
	m.updateViewport()
}

func (m *Model) updateViewport() {
	// Approximation, View() handles it precisely
	headerHeight := 10
	footerHeight := 4
	visibleHeight := m.height - headerHeight - footerHeight
	if visibleHeight <= 0 { visibleHeight = 5 }

	if m.cursor < m.top {
		m.top = m.cursor
	} else if m.cursor >= m.top+visibleHeight {
		m.top = m.cursor - visibleHeight + 1
	}
}

func (m *Model) sortFiles() {
	sort.Slice(m.files, func(i, j int) bool {
		// Dirs first
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
		if m.isHelp {
			if msg.String() == "q" || msg.String() == "esc" || msg.String() == "?" {
				m.isHelp = false
			}
			return m, nil
		}

		if m.isViewer {
			visibleHeight := m.height - 10
			if visibleHeight <= 0 { visibleHeight = 10 }

			switch msg.String() {
			case "q", "esc":
				m.isViewer = false
			case "o": // Open externally
				filesystem.OpenFile(m.viewerPath)
				m.isViewer = false
				m.message = "Opened externally: " + filepath.Base(m.viewerPath)
				return m, tick()
			case "j", "down":
				if m.viewerTop < len(m.viewerContent)-1 {
					m.viewerTop++
				}
			case "k", "up":
				if m.viewerTop > 0 {
					m.viewerTop--
				}
			case "pgdown":
				m.viewerTop += visibleHeight
				if m.viewerTop >= len(m.viewerContent) {
					m.viewerTop = max(0, len(m.viewerContent)-1)
				}
			case "pgup":
				m.viewerTop -= visibleHeight
				if m.viewerTop < 0 {
					m.viewerTop = 0
				}
			case "home":
				m.viewerTop = 0
			case "end":
				m.viewerTop = max(0, len(m.viewerContent)-visibleHeight)
			}
			return m, nil
		}

		if m.isInput {
			switch msg.String() {
			case "enter":
				m.isInput = false
				newPath := m.input.Value()
				
				// Handle Windows drive letters (e.g. "D:" -> "D:\")
				if len(newPath) == 2 && newPath[1] == ':' {
					newPath += "\\"
				}

				if absPath, err := filepath.Abs(newPath); err == nil {
					if _, err := os.Stat(absPath); err == nil {
						m.path = absPath
						m.cursor = 0
						m.top = 0
						m.loadFiles("")
					} else {
						m.message = "Invalid path: " + absPath
						return m, tick()
					}
				} else {
					m.message = "Invalid path format: " + newPath
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
				m.loadFiles("")
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

		case "?":
			m.isHelp = true
			return m, nil

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

		case "pgdown":
			headerHeight := 10
			footerHeight := 4
			visibleHeight := m.height - headerHeight - footerHeight
			if visibleHeight <= 0 { visibleHeight = 10 }

			m.cursor += visibleHeight
			if m.cursor >= len(m.files) {
				m.cursor = max(0, len(m.files)-1)
			}
			m.updateViewport()

		case "pgup":
			headerHeight := 10
			footerHeight := 4
			visibleHeight := m.height - headerHeight - footerHeight
			if visibleHeight <= 0 { visibleHeight = 10 }

			m.cursor -= visibleHeight
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.updateViewport()

		case "h", "left", "backspace":
			parent := filepath.Dir(m.path)
			if parent != m.path {
				oldDirName := filepath.Base(m.path)
				m.path = parent
				m.cursor = 0
				m.top = 0
				m.loadFiles(oldDirName)
			}

		case "home":
			m.cursor = 0
			m.updateViewport()

		case "end":
			if len(m.files) > 0 {
				m.cursor = len(m.files) - 1
				m.updateViewport()
			}

		case "l", "right", "enter":
			if len(m.files) > 0 {
				f := m.files[m.cursor]
				if f.IsDir {
					m.path = f.Path
					m.cursor = 0
					m.top = 0
					m.loadFiles("")
				} else {
					if f.Name != ".." {
						filesystem.OpenFile(f.Path)
						m.message = fmt.Sprintf("Opened %s", f.Name)
						return m, tick()
					}
				}
			}

		case "v": // View internally
			if len(m.files) > 0 {
				f := m.files[m.cursor]
				if !f.IsDir && f.Name != ".." {
					content, err := os.ReadFile(f.Path)
					if err == nil {
						m.isViewer = true
						m.viewerPath = f.Path
						m.viewerContent = strings.Split(string(content), "\n")
						m.viewerTop = 0
						return m, nil
					} else {
						m.message = "Could not read file: " + err.Error()
						return m, tick()
					}
				}
			}

		case "/": // Go to path
			m.isInput = true
			m.input.SetValue(m.path)
			m.input.Focus()
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
				m.loadFiles("")
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
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	if m.isHelp {
		return ui.MainBoxStyle.Width(m.width - 2).Height(m.height - 2).Render(m.HelpView())
	}

	if m.isViewer {
		return ui.MainBoxStyle.Width(m.width - 2).Height(m.height - 2).Render(m.ViewerView())
	}

	// 1. Header Section
	title := ui.HeaderStyle.Width(m.width - 6).Align(lipgloss.Center).Render("Atlas Conquistador")
	pathText := ui.PathStyle.Width(m.width - 6).Render("Path: " + m.path)
	
	var infoText string
	if m.err != nil {
		infoText = ui.WarningStyle.Render(fmt.Sprintf("⚠️ Error: %v", m.err))
	} else {
		infoText = ui.InfoStyle.Render(fmt.Sprintf("%d items • Sorted by %s", len(m.files), m.sorting))
	}
	
	headerView := lipgloss.JoinVertical(lipgloss.Left, title, pathText, infoText)
	headerBox := ui.HeaderBoxStyle.Width(m.width - 4).Render(headerView)

	// 2. Input/Confirm Section
	var middleHeader string
	if m.isInput {
		middleHeader = ui.SelectedStyle.Render("Go to: ") + m.input.View()
	} else if m.isConfirm {
		middleHeader = ui.WarningStyle.Render(fmt.Sprintf("Delete %d items? (y/n)", len(m.toDelete)))
	}

	// 3. Footer Section
	var footerText string
	if m.message != "" {
		footerText = ui.SuccessStyle.Render(" LOG: " + m.message)
	} else {
		footerText = ui.InfoStyle.Render("h/j/k/l: Navigate • ?: Help • q: Quit")
	}
	footerBox := ui.FooterBoxStyle.Width(m.width - 4).Render(footerText)

	// 4. File List Section - Precise height calculation
	// MainBox borders (2) + headerBox + footerBox
	occupiedHeight := 2 + lipgloss.Height(headerBox) + lipgloss.Height(footerBox)
	if middleHeader != "" {
		occupiedHeight += lipgloss.Height(middleHeader) + 1 // +1 for JoinVertical newline
	}
	
	visibleHeight := m.height - occupiedHeight
	if visibleHeight <= 0 { visibleHeight = 1 }

	end := m.top + visibleHeight
	if end > len(m.files) {
		end = len(m.files)
	}

	var fileListItems []string
	if len(m.files) == 0 && m.err == nil {
		fileListItems = append(fileListItems, "\n  (Empty directory)")
	}

	for i := m.top; i < end; i++ {
		f := m.files[i]
		indicator := "  "
		if m.cursor == i { indicator = "» " }

		selected := " "
		if m.selected[f.Path] { selected = ui.SelectedStyle.Render("●") }

		icon := ui.GetIcon(f.IsDir, f.Name)
		name := f.Name
		
		size := ""
		if !f.IsDir && f.Name != ".." { size = fmt.Sprintf("%d bytes", f.Size) }
		
		var nameStr string
		if f.IsDir {
			nameStr = ui.DirStyle.Render(name)
		} else {
			nameStr = ui.FileStyle.Render(name)
		}
		
		infoStr := ui.InfoStyle.Render(size)
		line := fmt.Sprintf("%s %s %s %-40s %s", indicator, selected, icon, nameStr, infoStr)
		
		if m.cursor == i {
			lineWidth := lipgloss.Width(line)
			padding := m.width - 6 - lineWidth 
			if padding < 0 { padding = 0 }
			line += strings.Repeat(" ", padding)
			fileListItems = append(fileListItems, ui.CursorStyle.Render(line))
		} else {
			fileListItems = append(fileListItems, line)
		}
	}

	// Fill remaining space
	for len(fileListItems) < visibleHeight {
		fileListItems = append(fileListItems, "")
	}

	fileListView := strings.Join(fileListItems, "\n")

	// Assemble final view
	var content string
	if middleHeader != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, headerBox, middleHeader, fileListView, footerBox)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left, headerBox, fileListView, footerBox)
	}

	return ui.MainBoxStyle.Width(m.width - 2).Height(m.height - 2).Render(content)
}

func (m Model) ViewerView() string {
	var s strings.Builder
	
	// Header
	header := ui.HeaderStyle.Render("Viewing: " + filepath.Base(m.viewerPath))
	headerBox := ui.HeaderBoxStyle.Width(m.width - 4).Render(header)
	s.WriteString(headerBox + "\n")

	// Content area calculation
	// MainBox(2) + headerBox + footerBox
	occupiedHeight := 2 + lipgloss.Height(headerBox) + 2 // 2 for footerBox
	viewerHeight := m.height - occupiedHeight
	if viewerHeight <= 0 { viewerHeight = 5 }
	
	end := m.viewerTop + viewerHeight
	if end > len(m.viewerContent) {
		end = len(m.viewerContent)
	}

	lnWidth := len(fmt.Sprintf("%d", len(m.viewerContent)))
	if lnWidth < 2 { lnWidth = 2 }

	renderedLines := 0
	for i := m.viewerTop; i < end; i++ {
		line := m.viewerContent[i]
		
		// Sanitize line (tabs break alignment, CR can break display)
		line = strings.ReplaceAll(line, "\t", "    ")
		line = strings.ReplaceAll(line, "\r", "")
		
		ln := ui.LineNumberStyle.Render(fmt.Sprintf("%*d", lnWidth, i+1))
		divider := ui.DividerStyle.Render(" │ ")

		// Truncate line safely (accounting for line numbers and divider)
		availableWidth := m.width - 6 - lnWidth - 3
		if availableWidth < 0 { availableWidth = 0 }
		
		// Use runes for safer truncation than bytes
		r := []rune(line)
		if len(r) > availableWidth {
			line = string(r[:max(0, availableWidth-3)]) + "..."
		}
		
		s.WriteString(" " + ln + divider + ui.FileStyle.Render(line) + "\n")
		renderedLines++
	}

	// EOF indicator
	if end == len(m.viewerContent) && renderedLines < viewerHeight {
		eof := ui.EOFStyle.Render(" EOF ")
		s.WriteString(" " + strings.Repeat(" ", lnWidth) + ui.DividerStyle.Render(" └ ") + eof + "\n")
		renderedLines++
	}

	// Padding
	for renderedLines < viewerHeight {
		s.WriteString("\n")
		renderedLines++
	}

	// Footer
	footerText := ui.InfoStyle.Render("j/k: Scroll • ") + 
		ui.SelectedStyle.Render("o: Open Externally") + 
		ui.InfoStyle.Render(" • ") + 
		ui.WarningStyle.Render("q/esc: Close")
	footerBox := ui.FooterBoxStyle.Width(m.width - 4).Render(footerText)
	s.WriteString(footerBox)

	return s.String()
}

func (m Model) HelpView() string {
	var s strings.Builder
	s.WriteString(ui.HeaderStyle.Render("Atlas Conquistador Help") + "\n\n")
	
	helpItems := [][]string{
		{"j/k, Arrows", "Navigate through files"},
		{"PgUp/PgDown", "Fast navigation / Page scroll"},
		{"Home/End", "Jump to start/end"},
		{"h, Left, Backspace", "Go to parent directory"},
		{"l, Right, Enter", "Open directory / Open externally"},
		{"v", "View file internally (plain text)"},
		{"Space", "Toggle selection"},
		{"/", "Go to specific path"},
		{"c", "Copy selected/current item"},
		{"x", "Cut selected/current item"},
		{"p", "Paste items from clipboard"},
		{"d", "Delete selected/current item (with confirmation)"},
		{"s", "Cycle sort order (Name -> Size -> Time)"},
		{"?", "Close help"},
		{"q, Esc", "Quit application / Close Help"},
	}

	for _, item := range helpItems {
		s.WriteString(fmt.Sprintf("%-20s %s\n", ui.SelectedStyle.Render(item[0]), ui.FileStyle.Render(item[1])))
	}

	s.WriteString("\n" + ui.InfoStyle.Render("Press any key to return..."))
	return s.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
