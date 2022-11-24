package file

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	WhiteoutPrefix = ".wh."
	OpaqueWhiteout = WhiteoutPrefix + WhiteoutPrefix + ".opq"
	DirSeparator   = "/"
)

// Path represents a file path
type Path string

// Normalize returns the cleaned file path representation (trimmed of spaces and resolve relative notations)
func (p Path) Normalize() Path {
	// note: when normalizing we cannot trim trailing whitespace since it is valid for a path to have suffix whitespace
	var trimmed = string(p)
	if strings.Count(trimmed, " ") < len(trimmed) {
		trimmed = strings.TrimLeft(string(p), " ")
	}

	// remove trailing dir separators
	trimmed = strings.TrimRight(trimmed, DirSeparator)

	// special case for root "/"
	if trimmed == "" {
		return DirSeparator
	}
	return Path(path.Clean(trimmed))
}

func (p Path) IsAbsolutePath() bool {
	return strings.HasPrefix(string(p), DirSeparator)
}

// Basename of the path (i.e. filename)
func (p Path) Basename() string {
	return path.Base(string(p))
}

// IsDirWhiteout indicates if the path has a basename is a opaque whiteout (which means all parent directory contents should be ignored during squashing)
func (p Path) IsDirWhiteout() bool {
	return p.Basename() == OpaqueWhiteout
}

// IsDirWhiteoutMount indicates if the path has a basename is a opaque whiteout (which means all parent directory contents should be ignored during squashing)
func (p Path) IsDirWhiteoutMount() bool {
	dir := filepath.Dir(string(p))

	var attr []byte
	_, err := unix.Getxattr(dir, "trusted.overlay.opaque", attr)
	if err != nil {
		return false
	}
	if string(attr) == "y" {
		return true
	}
	return false
}

// IsWhiteout indicates if the file basename has a whiteout prefix (which means that the file should be removed during squashing)
func (p Path) IsWhiteout() bool {
	return strings.HasPrefix(p.Basename(), WhiteoutPrefix)
}

// IsWhiteoutMount indicates if the file basename has a whiteout prefix (which means that the file should be removed during squashing)
func (p Path) IsWhiteoutMount() bool {
	return isCharDevice(string(p))
}

// isCharDevice Determine if a path exist and is a character input device.
func isCharDevice(path string) bool {
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	m := fi.Mode()
	if m&os.ModeCharDevice == 0 {
		return false
	}
	return true
}

// UnWhiteoutPath is a representation of the current path with no whiteout prefixes
func (p Path) UnWhiteoutPath() (Path, error) {
	basename := p.Basename()
	if strings.HasPrefix(basename, OpaqueWhiteout) {
		return p.ParentPath()
	}
	parent, err := p.ParentPath()
	if err != nil {
		return "", err
	}
	return Path(path.Join(string(parent), strings.TrimPrefix(basename, WhiteoutPrefix))), nil
}

// UnWhiteoutPathMount is a representation of the current path with no whiteout prefixes
func (p Path) UnWhiteoutPathMount() (Path, error) {
	if p.IsDirWhiteoutMount() {
		return p.ParentPath()
	}

	parent, err := p.ParentPath()
	if err != nil {
		return "", err
	}
	return Path(path.Join(string(parent))), nil

}

// ParentPath returns a path object to the current files parent directory (or errors out if there is no parent)
func (p Path) ParentPath() (Path, error) {
	parent, child := path.Split(string(p))
	sanitized := Path(parent).Normalize()
	if sanitized == "/" {
		if child != "" {
			return "/", nil
		}
		return "", fmt.Errorf("no parent")
	}
	return sanitized, nil
}

// AllPaths returns all constituent paths of the current path + the current path itself (e.g. /home/wagoodman/file.txt -> /, /home, /home/wagoodman, /home/wagoodman/file.txt )
func (p Path) AllPaths() []Path {
	fullPaths := p.ConstituentPaths()
	if p != "/" {
		fullPaths = append(fullPaths, p)
	}
	return fullPaths
}

// ConstituentPaths returns all constituent paths for the current path (not including the current path itself) (e.g. /home/wagoodman/file.txt -> /, /home, /home/wagoodman )
func (p Path) ConstituentPaths() []Path {
	parents := strings.Split(strings.Trim(string(p), DirSeparator), DirSeparator)
	fullPaths := make([]Path, len(parents))
	for idx := range parents {
		cur := DirSeparator + strings.Join(parents[:idx], DirSeparator)
		fullPaths[idx] = Path(cur)
	}
	return fullPaths
}

type Paths []Path

func (p Paths) Len() int           { return len(p) }
func (p Paths) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p Paths) Less(i, j int) bool { return string(p[i]) < string(p[j]) }
