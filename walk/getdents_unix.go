// Author:  Niels A.D.
// Project: autoindex (https://github.com/nielsAD/autoindex)
// License: Mozilla Public License, v2.0

package walk

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"unsafe"
)

func nameFromDent(de *syscall.Dirent) []byte {
	// Because this GOOS' syscall.Dirent does not provide a field that specifies
	// the name length, this function must first calculate the max possible name
	// length, and then search for the NULL byte.
	ml := int(uint64(de.Reclen) - uint64(unsafe.Offsetof(syscall.Dirent{}.Name)))

	// Convert syscall.Dirent.Name, which is array of int8, to []byte, by
	// overwriting Cap, Len, and Data slice header fields to values from
	// syscall.Dirent fields. Setting the Cap, Len, and Data field values for
	// the slice header modifies what the slice header points to, and in this
	// case, the name buffer.
	var name []byte
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&name))
	sh.Cap = ml
	sh.Len = ml
	sh.Data = uintptr(unsafe.Pointer(&de.Name[0]))

	if index := bytes.IndexByte(name, 0); index >= 0 {
		sh.Cap = index
		sh.Len = index
		return name
	}

	return nil
}

func getdents(name string, buf []byte) ([]Dirent, error) {
	dir, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	fd := int(dir.Fd())

	var res []Dirent
	for {
		n, err := syscall.Getdents(fd, buf)
		if err != nil {
			dir.Close()
			return nil, err
		}
		if n <= 0 {
			break
		}

		buf := buf[:n]
		for len(buf) > 0 {
			de := (*syscall.Dirent)(unsafe.Pointer(&buf[0]))
			buf = buf[de.Reclen:]

			if de.Ino == 0 {
				continue
			}

			nb := nameFromDent(de)
			nl := len(nb)
			if (nl == 0) || (nl == 1 && nb[0] == '.') || (nl == 2 && nb[0] == '.' && nb[1] == '.') {
				continue
			}
			entry := string(nb)

			var mode os.FileMode
			switch de.Type {
			case syscall.DT_REG:
				// regular file
			case syscall.DT_DIR:
				mode = os.ModeDir
			case syscall.DT_LNK:
				mode = os.ModeSymlink
			case syscall.DT_CHR:
				mode = os.ModeDevice | os.ModeCharDevice
			case syscall.DT_BLK:
				mode = os.ModeDevice
			case syscall.DT_FIFO:
				mode = os.ModeNamedPipe
			case syscall.DT_SOCK:
				mode = os.ModeSocket
			default:
				fi, err := os.Stat(filepath.Join(name, entry))
				if err != nil {
					dir.Close()
					return nil, err
				}
				mode = fi.Mode() & os.ModeType
			}

			res = append(res, Dirent{name: entry, modeType: mode})
		}
	}
	if err = dir.Close(); err != nil {
		return nil, err
	}
	return res, nil
}
