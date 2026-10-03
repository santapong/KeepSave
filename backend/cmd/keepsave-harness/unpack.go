package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/santapong/KeepSave/backend/internal/automation"
	"github.com/santapong/KeepSave/backend/internal/harness"
)

const maxPackageJSON = 1 << 20
const maxPackageFiles = 32
const maxPackageFile = 128 << 10
const maxPackageTotal = 256 << 10

var errPackageInput = errors.New("invalid or oversized candidate package")

func unpack(input, output string) (harness.Package, error) {
	raw, e := readCandidateJSON(input)
	if e != nil {
		return harness.Package{}, e
	}
	if !uniquePackageJSON(raw) {
		return harness.Package{}, errPackageInput
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var downloaded automation.Package
	if decoder.Decode(&downloaded) != nil {
		return harness.Package{}, errPackageInput
	}
	if len(downloaded.Files) == 0 || len(downloaded.Files) > maxPackageFiles {
		return harness.Package{}, errPackageInput
	}
	total := 0
	for name, content := range downloaded.Files {
		if name == "." || !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00\r\n") || len(content) > maxPackageFile {
			return harness.Package{}, errPackageInput
		}
		total += len(content)
		if total > maxPackageTotal {
			return harness.Package{}, errPackageInput
		}
	}
	p, e := harness.FromAutomation(downloaded)
	if e != nil {
		return harness.Package{}, e
	}
	if e = writeCandidateDirectory(output, p.Files); e != nil {
		return harness.Package{}, e
	}
	return p, nil
}

// Open and pin each real directory component rather than checking an absolute
// pathname and then reopening it. A concurrent rename cannot redirect a later
// operation into a substituted ancestor.
func candidateDirectoryRoot(path string) (*os.Root, error) {
	absolute, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	root, e := os.OpenRoot(string(filepath.Separator))
	if e != nil {
		return nil, e
	}
	for _, component := range strings.Split(strings.TrimPrefix(absolute, string(filepath.Separator)), string(filepath.Separator)) {
		if component == "" {
			continue
		}
		info, e := root.Lstat(component)
		if e != nil {
			root.Close()
			return nil, e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			root.Close()
			return nil, errors.New("candidate package ancestors must be real directories")
		}
		child, e := root.OpenRoot(component)
		root.Close()
		if e != nil {
			return nil, e
		}
		opened, e := child.Stat(".")
		if e != nil || !os.SameFile(info, opened) {
			child.Close()
			return nil, errPackageInput
		}
		root = child
	}
	return root, nil
}

func readCandidateJSON(path string) ([]byte, error) {
	root, e := candidateDirectoryRoot(filepath.Dir(path))
	if e != nil {
		return nil, e
	}
	defer root.Close()
	name := filepath.Base(path)
	before, e := root.Lstat(name)
	if e != nil {
		return nil, e
	}
	if !before.Mode().IsRegular() || before.Size() > maxPackageJSON {
		return nil, errPackageInput
	}
	f, e := root.Open(name)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	after, e := f.Stat()
	if e != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return nil, errPackageInput
	}
	raw, e := io.ReadAll(io.LimitReader(f, maxPackageJSON+1))
	if e != nil || len(raw) > maxPackageJSON {
		return nil, errPackageInput
	}
	return raw, nil
}

func writeCandidateDirectory(output string, files map[string][]byte) error {
	abs, e := filepath.Abs(output)
	if e != nil {
		return e
	}
	parent, name := filepath.Dir(abs), filepath.Base(abs)
	if name == "." || name == string(filepath.Separator) {
		return errPackageInput
	}
	parentRoot, e := candidateDirectoryRoot(parent)
	if e != nil {
		return e
	}
	defer parentRoot.Close()
	if e = parentRoot.Mkdir(name, 0700); e != nil {
		return e
	}
	created, e := parentRoot.Lstat(name)
	if e != nil {
		return e
	}
	root, e := parentRoot.OpenRoot(name)
	if e != nil {
		return e
	}
	defer root.Close()
	opened, e := root.Stat(".")
	if e != nil || !created.IsDir() || !os.SameFile(created, opened) {
		return errPackageInput
	}
	complete := false
	defer func() {
		if !complete {
			if current, e := parentRoot.Lstat(name); e == nil && os.SameFile(created, current) {
				_ = parentRoot.RemoveAll(name)
			}
		}
	}()
	ordered := make([]string, 0, len(files))
	for name := range files {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	for _, name := range ordered {
		if e = root.MkdirAll(filepath.Dir(filepath.FromSlash(name)), 0700); e != nil {
			return e
		}
		file, e := root.OpenFile(filepath.FromSlash(name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		_, writeErr := file.Write(files[name])
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	complete = true
	return nil
}

func uniquePackageJSON(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 24 {
			return errPackageInput
		}
		token, e := d.Token()
		if e != nil {
			return e
		}
		delimiter, composite := token.(json.Delim)
		if !composite {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return e
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return errPackageInput
				}
				seen[name] = true
				if e = walk(depth + 1); e != nil {
					return e
				}
			}
			end, e := d.Token()
			if e != nil || end != json.Delim('}') {
				return errPackageInput
			}
		case '[':
			for d.More() {
				if e = walk(depth + 1); e != nil {
					return e
				}
			}
			end, e := d.Token()
			if e != nil || end != json.Delim(']') {
				return errPackageInput
			}
		default:
			return errPackageInput
		}
		return nil
	}
	if walk(0) != nil {
		return false
	}
	_, e := d.Token()
	return e == io.EOF
}
