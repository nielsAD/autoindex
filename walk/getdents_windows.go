// Author:  Niels A.D.
// Project: autoindex (https://github.com/nielsAD/autoindex)
// License: Mozilla Public License, v2.0

package walk

import (
	"os"
)

func getdents(name string, _ []byte) ([]Dirent, error) {
	dir, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	r, err := dir.Readdir(0)
	if err != nil {
		dir.Close()
		return nil, err
	}
	if err := dir.Close(); err != nil {
		return nil, err
	}

	res := make([]Dirent, len(r))
	for i, info := range r {
		res[i] = Dirent{name: info.Name(), modeType: info.Mode() & os.ModeType}
	}

	return res, nil
}
