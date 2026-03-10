package explorer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"atlas.conquistador/internal/filesystem"
	"atlas.conquistador/internal/ui"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg struct{}
type opFinishedMsg struct {
	total int
	err   error
}

func tick() tea.Cmd {
	return tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func opTick() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg{} // Reuse tickMsg to trigger Update
	})
}

type OperationStatus struct {
	sync.Mutex
	done     int
	total    int
	lastFile string
	counting bool
	complete bool
	err      error
	duration time.Duration
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
	isHex      bool
	isCreate   bool
	isRename   bool
	createFile bool // true for file, false for folder
	toDelete   []string

	// Operation state
	isBusy       bool
	busyMsg      string
	pasteQueue   []string
	pasteIndex   int
	isConflict   bool
	conflictSrc  string
	conflictDst  string
	overwriteAll bool
	progressBar  progress.Model
	
	// Shared status for goroutine
	opStatus   *OperationStatus
	cancelFunc context.CancelFunc
	
	isCancelled bool
	doneItems   int
	totalItems  int

	// Viewer state
	viewerContent []string
	viewerPath    string
	viewerTop     int
}

func NewModel() Model {
	cwd, _ := os.Getwd()
	ti := textinput.New()
	ti.Placeholder = "Enter name..."
	ti.Focus()
	
	prog := progress.New(progress.WithDefaultGradient())
	
	m := Model{
		path:        cwd,
		selected:    make(map[string]bool),
		sorting:     "name",
		input:       ti,
		height:      20,
		progressBar: prog,
		opStatus:    &OperationStatus{},
	}
	m.loadFiles("")
	return m
}

func (m *Model) loadFiles(targetName string) {
	m.err = nil
	m.selected = make(map[string]bool)
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

	if hasParent {
		m.files = append([]filesystem.FileInfo{{
			Name:  "..",
			Path:  parent,
			IsDir: true,
		}}, m.files...)
	}

	m.sortFiles()

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
		// Parent directory always at the top
		if m.files[i].Name == ".." {
			return true
		}
		if m.files[j].Name == ".." {
			return false
		}

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

func (m *Model) resetOperationState() {
	m.isBusy = false
	m.busyMsg = ""
	m.pasteQueue = nil
	m.pasteIndex = 0
	m.isConflict = false
	m.conflictSrc = ""
	m.conflictDst = ""
	m.overwriteAll = false
	m.isCancelled = false
	m.doneItems = 0
	m.totalItems = 0
	m.progressBar.SetPercent(0)

	m.opStatus.Lock()
	m.opStatus.done = 0
	m.opStatus.total = 0
	m.opStatus.lastFile = ""
	m.opStatus.counting = false
	m.opStatus.complete = false
	m.opStatus.err = nil
	m.opStatus.Unlock()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tickMsg:
		if m.isBusy {
			m.opStatus.Lock()
			done, total, last := m.opStatus.done, m.opStatus.total, m.opStatus.lastFile
			counting := m.opStatus.counting
			complete := m.opStatus.complete
			m.opStatus.Unlock()

			if complete {
				m.busyMsg = "Done!"
				return m, nil // Stop polling
			}

			if counting {
				m.busyMsg = "Counting items..."
			} else {
				m.busyMsg = fmt.Sprintf("Processing: %s", last)
				if total > 0 {
					cmd = m.progressBar.SetPercent(float64(done) / float64(total))
				}
			}
			return m, tea.Batch(cmd, opTick())
		}
		m.message = ""
		return m, nil

	case opFinishedMsg:
		// Fallback for non-granular ops if any
		m.isBusy = false
		m.message = fmt.Sprintf("Operation complete: processed %d items", msg.total)
		m.loadFiles("")
		return m, tick()

	case progress.FrameMsg:
		newModel, cmd := m.progressBar.Update(msg)
		m.progressBar = newModel.(progress.Model)
		return m, cmd

	case tea.KeyMsg:
		if m.isBusy {
			m.opStatus.Lock()
			complete := m.opStatus.complete
			m.opStatus.Unlock()

			if complete && msg.String() == "enter" {
				m.isBusy = false
				if m.isCut {
					m.clipboard = nil
				}
				m.loadFiles("")
				return m, nil
			}

			if msg.String() == "ctrl+c" || msg.String() == "q" {
				if m.cancelFunc != nil {
					m.cancelFunc()
				}
				m.isBusy = false
				m.pasteQueue = nil
				m.message = "Operation cancelled"
				m.loadFiles("")
				return m, tick()
			}
			return m, nil
		}

		if m.isConflict {
			switch msg.String() {
			case "y", "Y":
				m.isConflict = false
				return m, m.startOperation(true)
			case "n", "N":
				m.isConflict = false
				m.pasteIndex++
				return m, m.nextPaste()
			case "a", "A":
				m.isConflict = false
				m.overwriteAll = true
				return m, m.startOperation(true)
			case "q", "esc":
				m.isConflict = false
				m.pasteQueue = nil
				m.isBusy = false
			}
			return m, nil
		}

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
				m.isHex = false
			case "o": // Open externally
				filesystem.OpenFile(m.viewerPath)
				m.isViewer = false
				m.isHex = false
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
				inputPath := m.input.Value()
				
				if len(inputPath) == 2 && inputPath[1] == ':' {
					inputPath += "\\"
				}

				var targetPath string
				if filepath.IsAbs(inputPath) {
					targetPath = inputPath
				} else {
					targetPath = filepath.Join(m.path, inputPath)
				}

				targetPath = filepath.Clean(targetPath)
				if _, err := os.Stat(targetPath); err == nil {
					m.path = targetPath
					m.cursor = 0
					m.top = 0
					m.loadFiles("")
				} else {
					m.message = "Invalid path: " + targetPath
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

		if m.isCreate {
			switch msg.String() {
			case "enter":
				name := m.input.Value()
				if name != "" {
					target := filepath.Join(m.path, name)
					var err error
					if m.createFile {
						err = filesystem.CreateFile(target)
					} else {
						err = filesystem.CreateDir(target)
					}
					if err != nil {
						m.message = "Error: " + err.Error()
					} else {
						m.message = "Created " + name
						m.loadFiles(name)
					}
				}
				m.isCreate = false
				m.input.SetValue("")
				return m, tick()
			case "tab":
				m.createFile = !m.createFile
			case "esc":
				m.isCreate = false
				m.input.SetValue("")
			default:
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		if m.isRename {
			switch msg.String() {
			case "enter":
				newName := m.input.Value()
				if newName != "" {
					oldPath := m.files[m.cursor].Path
					newPath := filepath.Join(m.path, newName)
					
					if _, err := os.Stat(newPath); err == nil {
						m.message = "Error: File already exists"
					} else {
						err := filesystem.Rename(oldPath, newPath)
						if err != nil {
							m.message = "Error: " + err.Error()
						} else {
							m.message = "Renamed to " + newName
							m.loadFiles(newName)
						}
					}
				}
				m.isRename = false
				m.input.SetValue("")
				return m, tick()
			case "esc":
				m.isRename = false
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
				m.isConfirm = false
				return m, m.startDeleteOperation()
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
			m.cursor = min(len(m.files)-1, m.cursor+10)
			m.updateViewport()

		case "pgup":
			m.cursor = max(0, m.cursor-10)
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
					m.openViewer(f.Path, false)
					return m, nil
				}
			}

		case "m": // Hex view
			if len(m.files) > 0 {
				f := m.files[m.cursor]
				if !f.IsDir && f.Name != ".." {
					m.openViewer(f.Path, true)
					return m, nil
				}
			}

		case "n": // New file/folder
			m.isCreate = true
			m.createFile = true
			m.input.SetValue("")
			m.input.Placeholder = "Enter name..."
			m.input.Focus()
			return m, nil

		case "r": // Rename
			if len(m.files) > 0 && m.files[m.cursor].Name != ".." {
				m.isRename = true
				m.input.SetValue(m.files[m.cursor].Name)
				m.input.Placeholder = "Enter new name..."
				m.input.Focus()
			}
			return m, nil

		case "/": // Go to path
			m.isInput = true
			m.input.SetValue(m.path)
			m.input.Placeholder = "Enter path..."
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
			m.clipboard = nil
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
			m.clipboard = nil
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
				m.pasteQueue = append([]string{}, m.clipboard...)
				m.pasteIndex = 0
				m.overwriteAll = false
				return m, m.nextPaste()
			}

		case "d": // Delete
			m.toDelete = nil
			if len(m.selected) > 0 {
				for p := range m.selected {
					m.toDelete = append(m.toDelete, p)
				}
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
		m.progressBar.Width = m.width - 10
	}

	return m, nil
}

func (m *Model) nextPaste() tea.Cmd {
	if m.pasteIndex >= len(m.pasteQueue) {
		return func() tea.Msg {
			return opFinishedMsg{total: len(m.pasteQueue)}
		}
	}

	src := m.pasteQueue[m.pasteIndex]
	dst := filepath.Join(m.path, filepath.Base(src))

	if _, err := os.Stat(dst); err == nil && !m.overwriteAll {
		m.isConflict = true
		m.conflictSrc = src
		m.conflictDst = dst
		return nil
	}

	return m.startOperation(true)
}

func (m *Model) startOperation(overwrite bool) tea.Cmd {
	m.isBusy = true
	m.busyMsg = "Initializing..."
	
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	
	queue := m.pasteQueue[m.pasteIndex:]
	destDir := m.path
	isCut := m.isCut
	
	m.opStatus.Lock()
	m.opStatus.done = 0
	m.opStatus.total = 0
	m.opStatus.lastFile = ""
	m.opStatus.counting = true
	m.opStatus.complete = false
	m.opStatus.err = nil
	m.opStatus.Unlock()

	startTime := time.Now()

	go func() {
		total := filesystem.CountItems(ctx, queue)
		m.opStatus.Lock()
		m.opStatus.total = total
		m.opStatus.counting = false
		m.opStatus.Unlock()

		done := 0
		onProgress := func(path string) {
			m.opStatus.Lock()
			done++
			m.opStatus.done = done
			m.opStatus.lastFile = filepath.Base(path)
			m.opStatus.Unlock()
		}

		for _, src := range queue {
			select {
			case <-ctx.Done():
				return
			default:
			}
			dst := filepath.Join(destDir, filepath.Base(src))
			var err error
			if isCut {
				err = filesystem.MoveWithProgress(ctx, src, dst, onProgress)
			} else {
				err = filesystem.CopyWithProgress(ctx, src, dst, onProgress)
			}
			if err != nil {
				m.opStatus.Lock()
				m.opStatus.err = err
				m.opStatus.Unlock()
				break
			}
		}
		
		m.opStatus.Lock()
		m.opStatus.complete = true
		m.opStatus.duration = time.Since(startTime)
		m.opStatus.Unlock()
	}()

	return opTick()
}

func (m *Model) startDeleteOperation() tea.Cmd {
	m.isBusy = true
	m.busyMsg = "Counting items..."
	
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	
	items := append([]string{}, m.toDelete...)
	m.toDelete = nil

	m.opStatus.Lock()
	m.opStatus.done = 0
	m.opStatus.total = 0
	m.opStatus.lastFile = ""
	m.opStatus.counting = true
	m.opStatus.complete = false
	m.opStatus.err = nil
	m.opStatus.Unlock()

	startTime := time.Now()

	go func() {
		total := filesystem.CountItems(ctx, items)
		m.opStatus.Lock()
		m.opStatus.total = total
		m.opStatus.counting = false
		m.opStatus.Unlock()

		done := 0
		onProgress := func(p string) {
			m.opStatus.Lock()
			done++
			m.opStatus.done = done
			m.opStatus.lastFile = filepath.Base(p)
			m.opStatus.Unlock()
		}

		var lastErr error
		for _, p := range items {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := filesystem.DeleteWithProgress(ctx, p, onProgress); err != nil {
				lastErr = err
				break
			}
		}

		m.opStatus.Lock()
		m.opStatus.complete = true
		m.opStatus.err = lastErr
		m.opStatus.duration = time.Since(startTime)
		m.opStatus.Unlock()
	}()

	return opTick()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (m *Model) openViewer(path string, hex bool) {
	f, err := os.Open(path)
	if err != nil {
		m.message = "Error: " + err.Error()
		return
	}
	defer f.Close()

	buf := make([]byte, 1024*100)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		m.message = "Error: " + err.Error()
		return
	}
	content := buf[:n]

	m.isViewer = true
	m.isHex = hex
	m.viewerPath = path
	m.viewerTop = 0

	if hex {
		m.viewerContent = formatHex(content)
	} else {
		isBinary := false
		for _, b := range content {
			if b == 0 {
				isBinary = true
				break
			}
		}

		if isBinary {
			m.viewerContent = []string{
				"⚠️  This file appears to be binary.",
				"",
				"Press 'm' to view in Hex Mode instead,",
				"or 'o' to open in your system's default app.",
			}
			return
		}

		raw := string(content)
		lines := strings.Split(raw, "\n")
		m.viewerContent = make([]string, len(lines))
		for i, line := range lines {
			line = strings.TrimSuffix(line, "\r")
			var sb strings.Builder
			for j, r := range line {
				if j > 1000 {
					sb.WriteString("...")
					break
				}
				if unicode.IsPrint(r) || r == '\t' {
					sb.WriteRune(r)
				} else {
					sb.WriteRune('.')
				}
			}
			m.viewerContent[i] = sb.String()
		}
	}
}

func formatHex(data []byte) []string {
	var lines []string
	for i := 0; i < len(data); i += 16 {
		end := i + 16
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]

		var hexPart strings.Builder
		for j := 0; j < 16; j++ {
			if j < len(chunk) {
				hexPart.WriteString(fmt.Sprintf("%02X ", chunk[j]))
			} else {
				hexPart.WriteString("   ")
			}
			if j == 7 {
				hexPart.WriteString(" ")
			}
		}

		var asciiPart strings.Builder
		for _, b := range chunk {
			if b >= 32 && b <= 126 {
				asciiPart.WriteByte(b)
			} else {
				asciiPart.WriteByte('.')
			}
		}

		lines = append(lines, fmt.Sprintf("%08X  %s |%s|", i, hexPart.String(), asciiPart.String()))
	}
	return lines
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	if m.isBusy {
		return ui.MainBoxStyle.Width(m.width - 2).Height(m.height - 2).Render(m.BusyView())
	}

	if m.isConflict {
		return ui.MainBoxStyle.Width(m.width - 2).Height(m.height - 2).Render(m.ConflictView())
	}

	if m.isHelp {
		return ui.MainBoxStyle.Width(m.width - 2).Height(m.height - 2).Render(m.HelpView())
	}

	if m.isViewer {
		return ui.MainBoxStyle.Width(m.width - 2).Height(m.height - 2).Render(m.ViewerView())
	}

	title := ui.HeaderStyle.Width(m.width - 6).Align(lipgloss.Center).Render("Atlas Conquistador")
	pathText := ui.PathStyle.Width(m.width - 6).Render("Path: " + m.path)
	
	clipboardInfo := ""
	if len(m.clipboard) > 0 {
		mode := "Copied"
		if m.isCut { mode = "Cut" }
		clipboardInfo = fmt.Sprintf(" • %s (%d)", ui.SelectedStyle.Render(mode), len(m.clipboard))
	}

	var infoText string
	if m.err != nil {
		infoText = ui.WarningStyle.Render(fmt.Sprintf("⚠️ Error: %v", m.err))
	} else {
		infoText = ui.InfoStyle.Render(fmt.Sprintf("%d items • Sorted by %s%s", len(m.files), m.sorting, clipboardInfo))
	}
	
	headerView := lipgloss.JoinVertical(lipgloss.Left, title, pathText, infoText)
	headerBox := ui.HeaderBoxStyle.Width(m.width - 4).Render(headerView)

	var middleHeader string
	if m.isInput {
		middleHeader = ui.SelectedStyle.Render("Go to: ") + m.input.View()
	} else if m.isConfirm {
		middleHeader = ui.WarningStyle.Render(fmt.Sprintf("Delete %d items? (y/n)", len(m.toDelete)))
	} else if m.isCreate {
		typeStr := "Folder"
		if m.createFile { typeStr = "File" }
		middleHeader = ui.SelectedStyle.Render(fmt.Sprintf("New %s: ", typeStr)) + m.input.View() + ui.InfoStyle.Render(" (Tab to toggle)")
	} else if m.isRename {
		middleHeader = ui.SelectedStyle.Render("Rename: ") + m.input.View()
	}

	var footerText string
	if m.message != "" {
		footerText = ui.SuccessStyle.Render(" LOG: " + m.message)
	} else {
		footerText = ui.InfoStyle.Render("h/j/k/l: Navigate • ?: Help • q: Quit")
	}
	footerBox := ui.FooterBoxStyle.Width(m.width - 4).Render(footerText)

	occupiedHeight := 2 + lipgloss.Height(headerBox) + lipgloss.Height(footerBox)
	if middleHeader != "" {
		occupiedHeight += lipgloss.Height(middleHeader) + 1 
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

	for len(fileListItems) < visibleHeight {
		fileListItems = append(fileListItems, "")
	}

	fileListView := strings.Join(fileListItems, "\n")

	var content string
	if middleHeader != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, headerBox, middleHeader, fileListView, footerBox)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left, headerBox, fileListView, footerBox)
	}

	return ui.MainBoxStyle.Width(m.width - 2).Height(m.height - 2).Render(content)
}

func (m Model) BusyView() string {
	var s strings.Builder
	m.opStatus.Lock()
	complete := m.opStatus.complete
	duration := m.opStatus.duration
	m.opStatus.Unlock()

	if complete {
		s.WriteString(ui.SuccessStyle.Render("Operation Complete!") + "\n\n")
		s.WriteString(ui.InfoStyle.Render(fmt.Sprintf("  All items processed in %v.", duration.Round(time.Millisecond))) + "\n\n")
		s.WriteString(ui.SelectedStyle.Render("  Press Enter to continue..."))
		return s.String()
	}

	s.WriteString(ui.HeaderStyle.Render("Operation in Progress") + "\n\n")
	s.WriteString("  " + m.busyMsg + "\n\n")
	
	s.WriteString("  " + m.progressBar.View() + "\n\n")
	
	s.WriteString(ui.InfoStyle.Render("  Please wait...") + "\n\n")
	s.WriteString(ui.WarningStyle.Render("  Press Ctrl+C or Q to force cancel"))
	return s.String()
}

func (m Model) ConflictView() string {
	var s strings.Builder
	s.WriteString(ui.WarningStyle.Render("File Conflict Detected") + "\n\n")
	s.WriteString(fmt.Sprintf("  File already exists in destination:\n  %s\n\n", filepath.Base(m.conflictDst)))
	s.WriteString("  " + ui.SelectedStyle.Render("y: Overwrite"))
	s.WriteString("  " + ui.FileStyle.Render("n: Skip"))
	s.WriteString("  " + ui.SuccessStyle.Render("a: Overwrite All"))
	s.WriteString("  " + ui.WarningStyle.Render("q: Cancel All"))
	return s.String()
}

func (m Model) ViewerView() string {
	var s strings.Builder
	
	headerText := "Viewing: " + filepath.Base(m.viewerPath)
	if m.isHex { headerText = "Hex View: " + filepath.Base(m.viewerPath) }
	header := ui.HeaderStyle.Render(headerText)
	headerBox := ui.HeaderBoxStyle.Width(m.width - 4).Render(header)
	s.WriteString(headerBox + "\n")

	occupiedHeight := 2 + lipgloss.Height(headerBox) + 2
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
		line = strings.ReplaceAll(line, "\t", "    ")
		line = strings.ReplaceAll(line, "\r", "")
		
		ln := ui.LineNumberStyle.Render(fmt.Sprintf("%*d", lnWidth, i+1))
		divider := ui.DividerStyle.Render(" │ ")

		availableWidth := m.width - 6 - lnWidth - 3
		if availableWidth < 0 { availableWidth = 0 }
		
		r := []rune(line)
		if len(r) > availableWidth {
			line = string(r[:max(0, availableWidth-3)]) + "..."
		}
		
		s.WriteString(" " + ln + divider + ui.FileStyle.Render(line) + "\n")
		renderedLines++
	}

	if end == len(m.viewerContent) && renderedLines < viewerHeight {
		eof := ui.EOFStyle.Render(" EOF ")
		s.WriteString(" " + strings.Repeat(" ", lnWidth) + ui.DividerStyle.Render(" └ ") + eof + "\n")
		renderedLines++
	}

	for renderedLines < viewerHeight {
		s.WriteString("\n")
		renderedLines++
	}

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
	title := ui.HeaderStyle.Width(m.width - 6).Align(lipgloss.Center).Render("Atlas Conquistador Help")
	s.WriteString(title + "\n\n")
	
	helpItems := [][]string{
		{"Key", "Action"},
		{"---", "---"},
		{"j/k, Arrows", "Navigate through files"},
		{"PgUp/PgDown", "Fast navigation / Page scroll"},
		{"Home/End", "Jump to start/end"},
		{"h, Left", "Go to parent directory"},
		{"l, Right, Enter", "Open directory / Open externally"},
		{"v", "View file internally (text)"},
		{"m", "Hex view file"},
		{"n", "New file or folder"},
		{"r", "Rename current item"},
		{"Space", "Toggle selection"},
		{"/", "Go to specific path (Rel/Abs)"},
		{"c", "Copy selected/current item"},
		{"x", "Cut selected/current item"},
		{"p", "Paste items from clipboard"},
		{"d", "Delete items (with confirm)"},
		{"s", "Cycle sort (Name/Size/Time)"},
		{"?", "Show/Close help"},
		{"q, Esc", "Quit / Close Help"},
	}

	// Calculate column widths
	col1Width := 20
	col2Width := m.width - 10 - col1Width

	for i, item := range helpItems {
		var k, a string
		if i == 0 {
			k = ui.SelectedStyle.Render(fmt.Sprintf("%-*s", col1Width, item[0]))
			a = ui.SelectedStyle.Render(item[1])
		} else if i == 1 {
			k = ui.DividerStyle.Render(strings.Repeat("─", col1Width))
			a = ui.DividerStyle.Render(strings.Repeat("─", col2Width))
		} else {
			k = ui.SelectedStyle.Render(fmt.Sprintf("%-*s", col1Width, item[0]))
			a = ui.FileStyle.Render(item[1])
		}
		
		s.WriteString(fmt.Sprintf("  %s %s %s\n", k, ui.DividerStyle.Render("│"), a))
	}

	s.WriteString("\n  " + ui.InfoStyle.Render("Press any key to return..."))
	return s.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
