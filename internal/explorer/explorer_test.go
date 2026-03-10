package explorer

import (
	"testing"
	"atlas.conquistador/internal/filesystem"
)

func TestSortFiles(t *testing.T) {
	m := Model{
		sorting: "name",
		files: []filesystem.FileInfo{
			{Name: "b", IsDir: true},
			{Name: "a", IsDir: true},
			{Name: "..", IsDir: true},
			{Name: "z.txt", IsDir: false},
		},
	}

	m.sortFiles()

	if m.files[0].Name != ".." {
		t.Errorf("Expected '..' at index 0, got %s", m.files[0].Name)
	}

	// Test with different sorting
	m.sorting = "size"
	m.files[0].Size = 0 // ".."
	m.files[1].Size = 100 // "a"
	m.files[2].Size = 200 // "b"
	
	m.sortFiles()
	if m.files[0].Name != ".." {
		t.Errorf("Expected '..' at index 0 when sorting by size, got %s", m.files[0].Name)
	}
	
	m.sorting = "time"
	m.sortFiles()
	if m.files[0].Name != ".." {
		t.Errorf("Expected '..' at index 0 when sorting by time, got %s", m.files[0].Name)
	}
}
