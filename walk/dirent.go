// Author:  Niels A.D.
// Project: autoindex (https://github.com/nielsAD/autoindex)
// License: Mozilla Public License, v2.0

package walk

import (
	"os"
)

// Dirent stores a single directory entry (see getDirents)
type Dirent struct {
	name     string
	modeType os.FileMode
}

// Name of the directory entry
func (d Dirent) Name() string {
	return d.name
}

// IsDir reports whether i describes a directory.
func (d Dirent) IsDir() bool {
	return d.modeType&os.ModeDir != 0
}

// IsRegular reports whether i describes a regular file.
func (d Dirent) IsRegular() bool {
	return d.modeType == 0
}

// IsSymlink reports whether i describes a symbolic link.
func (d Dirent) IsSymlink() bool {
	return d.modeType&os.ModeSymlink != 0
}
