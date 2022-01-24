// Author:  Niels A.D.
// Project: autoindex (https://github.com/nielsAD/autoindex)
// License: Mozilla Public License, v2.0

package walk_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/nielsAD/autoindex/walk"
)

func TestWalk(t *testing.T) {
	var expected = []string{".", "dirent.go", "getdents_stdlib.go", "getdents_unix.go", "walk.go", "walk_test.go"}

	var files []string
	walk.Walk(".", &walk.Options{
		Visit: func(dir string, entry *walk.Dirent) error {
			files = append(files, entry.Name())
			return nil
		},
		Error: func(dir string, entry *walk.Dirent, err error) error {
			t.Errorf("Error walking `%s`: %s\n", dir, err.Error())
			return err
		},
	})

	sort.Strings(files)
	if !reflect.DeepEqual(files, expected) {
		t.Errorf("Unexpected directory contents: %s\n", files)
	}
}
