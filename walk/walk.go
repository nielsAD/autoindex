// Author:  Niels A.D.
// Project: autoindex (https://github.com/nielsAD/autoindex)
// License: Mozilla Public License, v2.0

// The code in this package is loosely based on https://github.com/karrick/godirwalk

package walk

import (
	"errors"
	"os"
	"path/filepath"
)

// Errors
var (
	ErrNonDir = errors.New("walk: Cannot iterate non-directory")
)

// Options provide parameters for how the Walk function operates.
type Options struct {
	// Invoked before entering a (sub)directory.
	Enter Visitor

	// Invoked for every encountered directory entry.
	Visit Visitor

	// Invoked after leaving a (sub)directory.
	Leave ErrorHandler

	// Invoked on error.
	Error ErrorHandler

	// ScratchBuffer is an optional byte slice to use as a scratch buffer.
	ScratchBuffer []byte
}

// Visitor callback function
type Visitor func(dir string, entry *Dirent) error

// ErrorHandler callback function
type ErrorHandler func(dir string, entry *Dirent, err error) error

// DefaultScratchBufferSize specifies the size of the scratch buffer that will be allocated by Walk
const DefaultScratchBufferSize = 64 * 1024

var minScratchBufferSize = os.Getpagesize()

// Walk walks the file tree rooted at the specified directory, calling the
// specified callback function for each file system node in the tree, including
// root, symbolic links, and other node types.
func Walk(root string, options *Options) error {
	root = filepath.Clean(root)

	fi, err := os.Stat(root)
	if err != nil {
		return err
	}

	mode := fi.Mode()
	if mode&os.ModeDir == 0 {
		return ErrNonDir
	}

	dirent := Dirent{
		name:     filepath.Base(root),
		modeType: mode & os.ModeType,
	}

	if options.Enter == nil {
		options.Enter = defVisit
	}
	if options.Visit == nil {
		options.Visit = defVisit
	}
	if options.Leave == nil {
		options.Leave = defError
	}
	if options.Error == nil {
		options.Error = defError
	}

	if len(options.ScratchBuffer) < minScratchBufferSize {
		options.ScratchBuffer = make([]byte, DefaultScratchBufferSize)
	}

	return walk(root, &dirent, options)
}

func defVisit(dir string, entry *Dirent) error            { return nil }
func defError(dir string, entry *Dirent, err error) error { return err }

func walk(path string, dirent *Dirent, options *Options) error {
	if dirent.IsSymlink() {
		if !dirent.IsDir() {
			ref, err := os.Readlink(path)
			if err != nil {
				return err
			}

			if !filepath.IsAbs(ref) {
				ref = filepath.Join(filepath.Dir(path), ref)
			}

			s, err := os.Stat(ref)
			if err != nil {
				return err
			}
			dirent.modeType = (s.Mode() & os.ModeType) | os.ModeSymlink
		}
	}

	err := options.Visit(path, dirent)
	if err != nil {
		return err
	}

	if !dirent.IsDir() {
		return nil
	}

	if err := options.Enter(path, dirent); err != nil {
		if err == filepath.SkipDir {
			return nil
		}
		return err
	}

	ents, err := getdents(path, options.ScratchBuffer)
	if err != nil {
		err = options.Error(path, dirent, err)
		goto leave
	}

	for i := range ents {
		child := filepath.Join(path, ents[i].name)
		err = walk(child, &ents[i], options)
		if err == nil {
			continue
		}
		if err == filepath.SkipDir {
			break
		}

		err = options.Error(child, &ents[i], err)
		if err != nil {
			break
		}
	}

leave:
	if err == filepath.SkipDir {
		err = nil
	}
	return options.Leave(path, dirent, err)
}
