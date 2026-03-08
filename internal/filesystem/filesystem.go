package filesystem

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func OpenFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default: // linux, freebsd, etc.
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

type FileInfo struct {
	Name    string
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
	IsDir   bool
	Path    string
}

func ListDir(path string) ([]FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name:    entry.Name(),
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime(),
			IsDir:   entry.IsDir(),
			Path:    filepath.Join(path, entry.Name()),
		})
	}
	return files, nil
}

func CopyWithProgress(ctx context.Context, src, dst string, onProgress func(string)) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return copyDirWithProgress(ctx, src, dst, onProgress)
	}
	onProgress(src)
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}

func copyDirWithProgress(ctx context.Context, src, dst string, onProgress func(string)) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}
	onProgress(src)

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if err := CopyWithProgress(ctx, srcPath, dstPath, onProgress); err != nil {
			return err
		}
	}

	return nil
}

func MoveWithProgress(ctx context.Context, src, dst string, onProgress func(string)) error {
	onProgress(src)
	return os.Rename(src, dst)
}

func Rename(src, dst string) error {
	return os.Rename(src, dst)
}

func DeleteWithProgress(ctx context.Context, path string, onProgress func(string)) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := DeleteWithProgress(ctx, filepath.Join(path, entry.Name()), onProgress); err != nil {
				return err
			}
		}
	}
	
	onProgress(path)
	return os.Remove(path)
}

func CreateFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return f.Close()
}

func CreateDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func CountItems(ctx context.Context, paths []string) int {
	count := 0
	for _, p := range paths {
		filepath.Walk(p, func(_ string, info os.FileInfo, err error) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if err == nil {
				count++
			}
			return nil
		})
	}
	return count
}
