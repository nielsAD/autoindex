// Author:  Niels A.D.
// Project: autoindex (https://github.com/nielsAD/autoindex)
// License: Mozilla Public License, v2.0

package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	addr    = flag.String("a", ":80", "TCP network address to listen for connections")
	db      = flag.String("d", ":memory:", "Database location")
	dir     = flag.String("r", ".", "Root directory to serve")
	refresh = flag.String("i", "1h", "Refresh interval")
	cached  = flag.Bool("cached", false, "Serve everything from cache (rather than search/recursive queries only)")
)

var logOut = log.New(os.Stdout, "", 0)
var logErr = log.New(os.Stderr, "", 0)

func main() {
	flag.Parse()

	var interval time.Duration
	if *refresh != "" {
		i, err := time.ParseDuration(*refresh)
		if err != nil {
			logErr.Fatal(err)
		}
		interval = i
	}

	fs, err := New(*db, *dir)
	if err != nil {
		logErr.Fatal(err)
	}

	fs.Cached = *cached
	defer fs.Close()

	go func() {
		last := 0
		for {
			n, err := fs.Fill()
			if err != nil {
				logErr.Printf("Fill: %s\n", err.Error())
			}
			if n != last {
				logErr.Printf("%d records in database after update (%+d)\n", n, n-last)
				last = n
			}
			if interval == 0 {
				break
			}
			time.Sleep(interval)
		}
	}()

	srv := &http.Server{Addr: *addr}
	http.Handle("/", logRequest(fs))

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
		<-sig

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		srv.Shutdown(ctx)
	}()

	logErr.Printf("Serving files in '%s' on %s\n", *dir, *addr)
	logErr.Println(srv.ListenAndServe())

	fs.Close()
}

func orHyphen(s string) string {
	if s != "" {
		return s
	}
	return "-"
}

func logRequest(han http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _, _ := r.BasicAuth()
		h, _, _ := net.SplitHostPort(r.RemoteAddr)
		logOut.Printf("%s - %s [%s] \"%s %s %s\" 0 0 \"%s\" \"%s\"\n", h, orHyphen(u), time.Now().Format("02/Jan/2006:15:04:05 -0700"), r.Method, r.URL, r.Proto, orHyphen(r.Referer()), orHyphen(r.UserAgent()))
		han.ServeHTTP(w, r)
	})
}
