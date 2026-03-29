package explorer

import (
	"testing"
	tea "github.com/charmbracelet/bubbletea"
)

func TestInput(t *testing.T) {
	m := NewModel()
	
	// Simulate pressing 'f'
	m1, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = m1.(Model)
	
	if !m.isSearch {
		t.Fatal("Expected isSearch to be true")
	}
	
	// Simulate typing 'a'
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = m2.(Model)
	
	if m.filter != "a" {
		t.Fatalf("Expected filter to be 'a', got %q", m.filter)
	}

	if m.input.Value() != "a" {
		t.Fatalf("Expected input value to be 'a', got %q", m.input.Value())
	}
}
