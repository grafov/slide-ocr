package imageutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CollectImages walks a folder (or returns a single file) and lists image paths.
func CollectImages(paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		p = filepath.Clean(p)
		info, err := os.Stat(p)
		if err != nil {
			return out, err
		}
		if !info.IsDir() {
			if IsImagePath(p) {
				out = append(out, p)
			}
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") && path != p {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if IsImagePath(path) {
				out = append(out, path)
			}
			return nil
		})
		if err != nil {
			return out, err
		}
	}
	return out, nil
}
