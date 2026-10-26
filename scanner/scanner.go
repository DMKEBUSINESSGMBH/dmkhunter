package scanner

import (
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

type Scanner struct {
	paths   []string
	Ignores *[]string
}

func NewScanner(paths []string, ignores *[]string) Scanner {
	return Scanner{
		paths:   paths,
		Ignores: ignores,
	}
}

// Walk calls fn for every regular file in the configured paths that is not ignored.
// Symlinks are followed, but every file is passed only once: the real directories of all configured
// paths are walked first, links after that, so a file is reported under its real path when it is in
// a configured path, and under the first link to it otherwise.
// Ignore patterns are matched against the path relative to the configured path, e.g. "var/log/dev.log".
// Files are passed as soon as they are found, so callers can start working before the walk is done.
func (s Scanner) Walk(fn func(f *model.File) error) error {
	ignores, err := s.compileIgnores()

	if err != nil {
		return err
	}

	w := walker{ignores: ignores, fn: fn, dirs: make(map[fileID]walkedDir), files: make(map[fileID]bool)}

	for _, root := range s.paths {
		// a missing root or one with a broken link is an error, broken entries below it are skipped
		info, err := os.Stat(root)

		if err != nil {
			return err
		}

		// walked paths are built with filepath.Join and start with the cleaned root, see relativePath
		clean := filepath.Clean(root)

		switch {
		case info.IsDir():
			if err := w.walkDir(clean, root, info); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			// its directory may be walked through a link, so it is checked like a link to a file
			w.fileLinks = append(w.fileLinks, link{root: clean, path: root, info: info})
		}
	}

	// directory links can lead to more links, they are appended while the queue is worked off
	for i := 0; i < len(w.dirLinks); i++ {
		l := w.dirLinks[i]

		if err := w.walkDir(l.root, l.path, l.info); err != nil {
			return err
		}
	}

	// file links last, their directory may have been walked through a directory link
	for _, l := range w.fileLinks {
		if err := w.linkedFile(l); err != nil {
			return err
		}
	}

	return nil
}

// fileID identifies a file or directory independent of the path it was reached by.
type fileID struct {
	dev, ino uint64
}

func idOf(info fs.FileInfo) (fileID, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)

	if !ok {
		return fileID{}, false
	}

	return fileID{dev: uint64(st.Dev), ino: uint64(st.Ino)}, true
}

// walkedDir is the path a directory was walked under.
type walkedDir struct {
	root string
	path string
}

// link is a symlink found during the walk, info is its resolved target.
type link struct {
	root string
	path string
	info fs.FileInfo
}

type walker struct {
	ignores []*regexp.Regexp
	fn      func(f *model.File) error

	dirs      map[fileID]walkedDir
	files     map[fileID]bool // files reached through a link
	dirLinks  []link
	fileLinks []link
}

// walkDir passes the regular files in path to fn and descends into real subdirectories.
// Symlinks are queued, root is the configured path that ignore patterns are matched relative to.
func (w *walker) walkDir(root, path string, info fs.FileInfo) error {
	entries := w.readDir(root, path, info)

	for _, e := range entries {
		child := filepath.Join(path, e.Name())

		switch {
		case e.Type().IsRegular():
			if err := w.file(root, child); err != nil {
				return err
			}
		case e.IsDir():
			// entries removed during the walk are skipped
			info, err := e.Info()

			if err != nil {
				continue
			}

			if err := w.walkDir(root, child, info); err != nil {
				return err
			}
		case e.Type()&fs.ModeSymlink != 0:
			// broken links are skipped
			info, err := os.Stat(child)

			if err != nil {
				continue
			}

			if info.IsDir() {
				w.dirLinks = append(w.dirLinks, link{root: root, path: child, info: info})
			} else if info.Mode().IsRegular() {
				w.fileLinks = append(w.fileLinks, link{root: root, path: child, info: info})
			}
		}
		// sockets, devices, pipes etc. are skipped
	}

	return nil
}

// readDir returns the sorted entries of path, or nothing if it is unreadable or was already walked.
// os.ReadDir is used over os.Open and Stat on the handle, os.Open sets up non-blocking IO which costs more.
func (w *walker) readDir(root, path string, info fs.FileInfo) []fs.DirEntry {
	// each directory is walked once, this also ends link loops
	if id, ok := idOf(info); ok {
		if _, ok := w.dirs[id]; ok {
			return nil
		}
		w.dirs[id] = walkedDir{root: root, path: path}
	}

	entries, err := os.ReadDir(path)

	if err != nil {
		return nil
	}

	return entries
}

// linkedFile passes the target of l to fn, unless it was already passed.
// A target that was ignored in a walked directory is passed under the link path if that is not ignored.
func (w *walker) linkedFile(l link) error {
	real, ok := linkTarget(l.path)

	if !ok {
		return nil
	}

	if dir, err := os.Stat(rawDir(real)); err == nil {
		if id, ok := idOf(dir); ok {
			if d, walked := w.dirs[id]; walked && !w.ignored(d.root, filepath.Join(d.path, filepath.Base(real))) {
				return nil
			}
		}
	}

	if id, ok := idOf(l.info); ok {
		if w.files[id] {
			return nil
		}
		w.files[id] = true
	}

	return w.file(l.root, l.path)
}

// linkTarget follows the links starting at path to the file they end at.
// Unlike filepath.EvalSymlinks it doesn't resolve every directory on the way, which costs a syscall each,
// the returned path is only good for its name and for looking up its directory.
func linkTarget(path string) (string, bool) {
	// the kernel gives up after 40 links as well
	for range 40 {
		dest, err := os.Readlink(path)

		if err != nil {
			// path is no link (anymore), only a root that is a regular file gets here on the first round
			return path, true
		}

		// not cleaned on purpose, ".." after a linked directory must be resolved by the kernel
		if !filepath.IsAbs(dest) {
			dest = rawDir(path) + "/" + dest
		}

		info, err := os.Lstat(dest)

		if err != nil {
			return "", false
		}

		if info.Mode()&fs.ModeSymlink == 0 {
			return dest, true
		}

		path = dest
	}

	return "", false
}

// rawDir is filepath.Dir without cleaning the result.
func rawDir(path string) string {
	i := strings.LastIndexByte(path, '/')

	switch {
	case i < 0:
		return "."
	case i == 0:
		return "/"
	}

	return path[:i]
}

func (w *walker) file(root, path string) error {
	if w.ignored(root, path) {
		return nil
	}

	return w.fn(model.NewFile(path))
}

func (w *walker) ignored(root, path string) bool {
	// saves building the relative path for every file
	if len(w.ignores) == 0 {
		return false
	}

	return pathIsIgnored(w.ignores, relativePath(root, path))
}

func (s Scanner) Scan() map[string]*model.File {
	files := make(map[string]*model.File)

	s.Walk(func(f *model.File) error {
		files[f.Path] = f

		return nil
	})

	return files
}

func (s Scanner) compileIgnores() ([]*regexp.Regexp, error) {
	if s.Ignores == nil {
		return nil, nil
	}

	ignores := make([]*regexp.Regexp, 0, len(*s.Ignores))

	for _, pattern := range *s.Ignores {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		ignores = append(ignores, re)
	}

	return ignores, nil
}

// relativePath returns path relative to the scanned root with "/" separators, ignore patterns are matched against it.
func relativePath(root, path string) string {
	// fast path for the paths the walk builds below a cleaned root, filepath.Rel cleans both again
	if filepath.Separator == '/' {
		if rest, ok := strings.CutPrefix(path, root); ok && rest != "" {
			if strings.HasSuffix(root, "/") {
				return rest
			}
			if rel, ok := strings.CutPrefix(rest, "/"); ok && rel != "" {
				return rel
			}
		}
	}

	rel, err := filepath.Rel(root, path)

	if err != nil {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(rel)
}

// test if file matches ignore list
func pathIsIgnored(ignores []*regexp.Regexp, path string) bool {
	for _, re := range ignores {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}
