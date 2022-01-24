// Author:  Niels A.D.
// Project: autoindex (https://github.com/nielsAD/autoindex)
// License: Mozilla Public License, v2.0

//go:build !freebsd && !linux && !netbsd && !openbsd
// +build !freebsd,!linux,!netbsd,!openbsd

package walk

import (
	"os"
)

func getdents(name string, _ []byte) ([]Dirent, error) {
	dir, err := os.ReadDir(name)
	if err != nil {
		return nil, err
	}

	res := make([]Dirent, len(dir))
	for i, info := range dir {
		res[i] = Dirent{name: info.Name(), modeType: info.Type() & os.ModeType}
	}

	return res, nil
}
