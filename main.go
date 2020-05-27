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
	"path"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ulule/limiter"
	"github.com/ulule/limiter/drivers/middleware/stdlib"
	"github.com/ulule/limiter/drivers/store/memory"
)

var (
	addr      = flag.String("a", ":80", "TCP network address to listen for connections")
	db        = flag.String("d", ":memory:", "Database location")
	dir       = flag.String("r", ".", "Root directory to serve")
	refresh   = flag.String("i", "1h", "Refresh interval")
	ratelimit = flag.Int64("l", 5, "Request rate limit (req/sec per IP)")
	timeout   = flag.Duration("t", time.Second, "Request timeout")
	forwarded = flag.Bool("forwarded", false, "Trust X-Real-IP and X-Forwarded-For headers")
	cached    = flag.Bool("cached", false, "Serve everything from cache (rather than search/recursive queries only)")
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

	fs.Timeout = *timeout
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

	pub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Join("./public/", r.URL.Path)
		if s, err := os.Stat(p); err == nil && !s.IsDir() {
			http.ServeFile(w, r, p)
		} else {
			http.ServeFile(w, r, "./public/index.html")
		}
	})

	limit := stdlib.NewMiddleware(limiter.New(memory.NewStore(), limiter.Rate{Period: time.Second, Limit: *ratelimit}))

	srv := &http.Server{Addr: *addr}
	handleDefault := func(p string, h http.Handler) { http.Handle(p, realIP(*forwarded, checkMethod(h))) }
	handleLimited := func(p string, h http.Handler) { handleDefault(p, limit.Handler(logRequest(http.StripPrefix(p, h)))) }

	handleLimited("/idx/", fs)
	handleLimited("/dl/", nodir(http.FileServer(http.Dir(fs.Root))))
	handleLimited("/urllist.txt", http.HandlerFunc(fs.Sitemap))
	handleDefault("/", pub)

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

func checkMethod(han http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" && r.Method != "GET" {
			http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		han.ServeHTTP(w, r)
	})
}

func realIP(trustForward bool, han http.Handler) http.Handler {
	if !trustForward {
		return han
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if realHost := r.Header.Get("X-Forwarded-Host"); realHost != "" {
			r.Host = realHost
		}
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			r.RemoteAddr = realIP + ":0"
		}

		han.ServeHTTP(w, r)
	})
}

func logRequest(han http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _, _ := r.BasicAuth()
		h, _, _ := net.SplitHostPort(r.RemoteAddr)
		logOut.Printf("%s - %s [%s] \"%s %s %s\" 0 0 \"%s\" \"%s\"\n", h, orHyphen(u), time.Now().Format("02/Jan/2006:15:04:05 -0700"), r.Method, r.URL, r.Proto, orHyphen(r.Referer()), orHyphen(r.UserAgent()))
		han.ServeHTTP(w, r)
	})
}

func nodir(han http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}

		han.ServeHTTP(w, r)
	})
}
