// Author:  Niels A.D.
// Project: autoindex (https://github.com/nielsAD/autoindex)
// License: Mozilla Public License, v2.0

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nielsAD/autoindex/walk"
)

// CachedFS struct
type CachedFS struct {
	ql     *sql.Stmt
	qd     *sql.Stmt
	qs     *sql.Stmt
	db     *sql.DB
	dbr    int32
	dbp    string
	Root   string
	Cached bool
}

// New CachedFS
func New(dbp string, root string) (*CachedFS, error) {
	r, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbp)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS dirs (path TEXT);
		CREATE TABLE IF NOT EXISTS files (root INTEGER, name TEXT, dir BOOLEAN)
	`); err != nil {
		db.Close()
		return nil, err
	}

	ql, err := db.Prepare("SELECT dirs.path FROM dirs LIMIT 50000")
	if err != nil {
		db.Close()
		return nil, err
	}

	qd, err := db.Prepare("SELECT dirs.rowid FROM dirs WHERE path GLOB ? LIMIT 1")
	if err != nil {
		db.Close()
		return nil, err
	}

	qs, err := db.Prepare("SELECT dirs.path, files.name, files.dir FROM files LEFT JOIN dirs ON files.root = dirs.rowid WHERE files.root IN (SELECT rowid FROM dirs WHERE path GLOB ?) AND files.name LIKE ? ESCAPE '`' LIMIT 1000")
	if err != nil {
		db.Close()
		return nil, err
	}

	fs := CachedFS{
		ql:   ql,
		qd:   qd,
		qs:   qs,
		db:   db,
		dbp:  dbp,
		Root: r,
	}

	// Check if database already has root entry
	var id int64
	if fs.qd.QueryRow("/").Scan(&id) == nil {
		fs.dbr++
	}

	return &fs, nil
}

// Close closes the database, releasing any open resources.
func (fs *CachedFS) Close() error {
	return fs.db.Close()
}

const (
	insDir  = "INSERT INTO dirs_tmp (path) VALUES (?)"
	insFile = "INSERT INTO files_tmp (root, name, dir) VALUES (?, ?, ?)"
)

// Fill database
func (fs *CachedFS) Fill() (int, error) {
	if _, err := fs.db.Exec(`
		DROP TABLE IF EXISTS dirs_tmp;
		DROP TABLE IF EXISTS files_tmp;
		CREATE TABLE dirs_tmp (path TEXT);
		CREATE TABLE files_tmp (root INTEGER, name TEXT, dir BOOLEAN)
	`); err != nil {
		return 0, err
	}

	tx, err := fs.db.Begin()
	if err != nil {
		return 0, err
	}
	idir, err := tx.Prepare(insDir)
	if err != nil {
		return 0, err
	}
	ifile, err := tx.Prepare(insFile)
	if err != nil {
		return 0, err
	}

	cnt := 0
	dirs := []int64{0}
	trim := len(fs.Root)
	err = walk.Walk(fs.Root, &walk.Options{
		Error: func(r string, e *walk.Dirent, err error) error {
			logErr.Printf("Error iterating \"%s\": %s\n", r, err.Error())
			return nil
		},
		Visit: func(r string, e *walk.Dirent) error {
			// Skip root
			if cnt == 0 {
				cnt++
				return nil
			}

			n := e.Name()
			if n == "" || strings.HasPrefix(n, ".") {
				return nil
			}

			if _, err := ifile.Exec(dirs[len(dirs)-1], n, e.IsDir()); err != nil {
				return err
			}

			cnt++
			if cnt%16384 == 0 {
				if err := tx.Commit(); err != nil {
					return err
				}
				tx, err = fs.db.Begin()
				if err != nil {
					return err
				}
				idir, err = tx.Prepare(insDir)
				if err != nil {
					return err
				}
				ifile, err = tx.Prepare(insFile)
				if err != nil {
					return err
				}
			}

			return nil
		},
		Enter: func(r string, e *walk.Dirent) error {
			if strings.HasPrefix(e.Name(), ".") {
				return filepath.SkipDir
			}

			if e.IsSymlink() {
				e, err := filepath.EvalSymlinks(r)
				if err != nil {
					return err
				}
				a, err := filepath.Abs(e)
				if err != nil {
					return err
				}
				if strings.HasPrefix(a, fs.Root) {
					logErr.Printf("Skipping symlink relative to root (%s)\n", r)
					return filepath.SkipDir
				}
			}

			row, err := idir.Exec(r[trim:] + "/")
			if err != nil {
				return err
			}

			id, err := row.LastInsertId()
			if err != nil {
				return err
			}

			dirs = append(dirs, id)
			return nil
		},
		Leave: func(r string, e *walk.Dirent, err error) error {
			dirs = dirs[:len(dirs)-1]
			return err
		},
	})
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	if _, err := tx.Exec(`
		DROP TABLE IF EXISTS dirs;
		DROP TABLE IF EXISTS files;
		ALTER TABLE dirs_tmp RENAME TO dirs;
		ALTER TABLE files_tmp RENAME TO files;
		CREATE INDEX idx_dirs ON dirs (path);
		CREATE INDEX idx_files ON files (root);
	`); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	fs.db.Exec("VACUUM; PRAGMA shrink_memory")
	atomic.AddInt32(&fs.dbr, 1)

	return cnt, nil
}

// DBReady returns whether the DB is ready for querying
func (fs *CachedFS) DBReady() bool {
	return fs.db != nil && atomic.LoadInt32(&fs.dbr) != 0
}

var (
	escGlob  = regexp.MustCompile("[][*?]")
	escLike  = regexp.MustCompile("[%_`]")
	escSpace = regexp.MustCompile("\\s+")
)

func escapeGlob(s string) string {
	return escGlob.ReplaceAllStringFunc(s, func(m string) string { return "[" + m + "]" })
}

func escapeLike(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "%"
	}

	s = escLike.ReplaceAllStringFunc(s, func(m string) string { return "`" + m })
	s = escSpace.ReplaceAllString(s, "%")
	return "%" + s + "%"
}

func escapeRegex(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ".*"
	}
	s = regexp.QuoteMeta(s)
	s = escSpace.ReplaceAllString(s, ".*")
	return ".*" + s + ".*"
}

func cleanPath(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = path.Clean(p)
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// File data sent to client
type File struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Files list (sortable)
type Files []File

func (f Files) Len() int      { return len(f) }
func (f Files) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f Files) Less(i, j int) bool {
	if f[i].Type == f[j].Type {
		return strings.ToLower(f[i].Name) < strings.ToLower(f[j].Name)
	}
	return f[i].Type < f[j].Type
}

func (fs *CachedFS) serveCache(w http.ResponseWriter, r *http.Request) {
	if !fs.DBReady() {
		http.Error(w, "503 service unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	p := cleanPath(r.URL.Path)
	trim := len(p)

	p = escapeGlob(p)
	if r.URL.Query().Get("r") != "" {
		p += "*"
	}

	var id int64
	if fs.qd.QueryRowContext(ctx, p).Scan(&id) != nil {
		http.NotFound(w, r)
		return
	}

	resp := make(Files, 0)
	search := escapeLike(r.URL.Query().Get("q"))

	rows, err := fs.qs.QueryContext(ctx, p, search)
	if err != nil {
		goto interr
	}
	defer rows.Close()

	for rows.Next() {
		var root string
		var name string
		var dir bool
		err = rows.Scan(&root, &name, &dir)
		if err != nil {
			goto interr
		}

		f := File{Name: root[trim:] + name}
		if dir {
			f.Type = "d"
		} else {
			f.Type = "f"
		}

		resp = append(resp, f)
	}
	err = rows.Err()
	if err != nil {
		goto interr
	}

	sort.Sort(resp)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "max-age=60")
	json.NewEncoder(w).Encode(resp)
	return

interr:
	h, _, _ := net.SplitHostPort(r.RemoteAddr)
	logErr.Printf("%s \"%s %s %s\" \"%s\"\n", h, r.Method, r.URL, r.Proto, err.Error())
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (fs *CachedFS) serveLive(w http.ResponseWriter, r *http.Request) {
	p := cleanPath(filepath.Join(fs.Root, r.URL.Path))

	resp := make(Files, 0)
	search, err := regexp.Compile(escapeRegex(r.URL.Query().Get("q")))

	if err == nil {
		trim := len(p)
		depth := 0
		err = walk.Walk(p, &walk.Options{
			Error: func(r string, e *walk.Dirent, err error) error {
				logErr.Printf("Error iterating \"%s\": %s\n", r, err.Error())
				return nil
			},
			Visit: func(r string, e *walk.Dirent) error {
				if depth == 0 {
					return nil
				}

				n := e.Name()
				if n == "" || strings.HasPrefix(n, ".") || !search.MatchString(n) {
					return nil
				}

				f := File{Name: r[trim:]}
				if e.IsDir() {
					f.Type = "d"
				} else {
					f.Type = "f"
				}

				resp = append(resp, f)

				return nil
			},
			Enter: func(r string, e *walk.Dirent) error {
				if depth >= 1 {
					return filepath.SkipDir
				}
				depth++
				return nil
			},
			Leave: func(r string, e *walk.Dirent, err error) error {
				depth--
				return err
			},
		})
	}

	if err != nil {
		h, _, _ := net.SplitHostPort(r.RemoteAddr)
		if err == walk.ErrNonDir || os.IsNotExist(err) || os.IsPermission(err) {
			http.NotFound(w, r)
		} else {
			logErr.Printf("%s \"%s %s %s\" \"%s\"\n", h, r.Method, r.URL, r.Proto, err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	sort.Sort(resp)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "max-age=60")
	json.NewEncoder(w).Encode(resp)
}

func (fs *CachedFS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if fs.Cached || r.URL.Query().Get("r") != "" {
		fs.serveCache(w, r)
	} else {
		fs.serveLive(w, r)
	}
}

// Sitemap serves a list of all directories
func (fs *CachedFS) Sitemap(w http.ResponseWriter, r *http.Request) {
	if !fs.DBReady() {
		http.Error(w, "503 service unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	rows, err := fs.ql.QueryContext(ctx)
	if err != nil {
		goto interr
	}
	defer rows.Close()

	for rows.Next() {
		var path string
		err = rows.Scan(&path)
		if err != nil {
			goto interr
		}

		w.Write([]byte("https://"))
		w.Write([]byte(r.Host))
		w.Write([]byte(path[:len(path)-1]))
		w.Write([]byte{'\n'})
	}
	err = rows.Err()
	if err != nil {
		goto interr
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "max-age=3600")
	return

interr:
	h, _, _ := net.SplitHostPort(r.RemoteAddr)
	logErr.Printf("%s \"%s %s %s\" \"%s\"\n", h, r.Method, r.URL, r.Proto, err.Error())
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
