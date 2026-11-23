package site

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

// Handler serves a built site from dir as the host does: a path that ends in / serves its index.html, and a missing file serves 404.html with the status 404. The handler reads the files through an os.Root, so a request cannot read a file outside dir.
func Handler(dir string) (http.Handler, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	site := root.FS()
	files := http.FileServerFS(site)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if strings.HasSuffix(r.URL.Path, "/") {
			name = path.Join(name, "index.html")
		}
		if name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) {
			notFound(w, site)
			return
		}
		if _, err := fs.Stat(site, name); err != nil {
			notFound(w, site)
			return
		}
		if strings.HasSuffix(name, ".md") {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		}
		files.ServeHTTP(w, r)
	}), nil
}

func notFound(w http.ResponseWriter, site fs.FS) {
	data, err := fs.ReadFile(site, "404.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err == nil {
		_, _ = w.Write(data)
	}
}
