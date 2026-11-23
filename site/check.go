package site

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The limits of Cloudflare static assets, with a margin.
const (
	MaxFiles    = 19500
	MaxFileSize = 25 << 20
)

// Report is the result of Check.
type Report struct {
	Files    int
	Bytes    int64
	Problems []string
}

// OK reports whether the check found no problem.
func (r *Report) OK() bool { return len(r.Problems) == 0 }

var attrRE = regexp.MustCompile(`\s(?:href|src)="([^"]*)"`)

// Check checks a built site in dir: the number and the size of the files, each link from a page to the site, and the math.
func Check(dir string) (*Report, error) {
	r := &Report{}
	files := map[string]bool{}
	var pages []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(p, dir+string(filepath.Separator)))
		files[rel] = true
		r.Files++
		r.Bytes += info.Size()
		if info.Size() > MaxFileSize {
			r.Problems = append(r.Problems, fmt.Sprintf("%s: %d bytes is more than the limit of %d", rel, info.Size(), MaxFileSize))
		}
		if strings.HasSuffix(rel, ".html") {
			pages = append(pages, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if r.Files > MaxFiles {
		r.Problems = append(r.Problems, fmt.Sprintf("the site has %d files, more than the limit of %d", r.Files, MaxFiles))
	}
	for _, need := range []string{"index.html", "404.html", "api/search.json", "llms.txt"} {
		if !files[need] {
			r.Problems = append(r.Problems, need+": the file is missing")
		}
	}
	sort.Strings(pages)
	for _, p := range pages {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p))) // #nosec G304 -- a file of the built site
		if err != nil {
			return nil, err
		}
		text := string(data)
		if strings.Contains(text, `class="tex-error"`) {
			r.Problems = append(r.Problems, p+": a formula does not render")
		}
		for _, m := range attrRE.FindAllStringSubmatch(text, -1) {
			if target, ok := internal(p, m[1]); ok && !exists(files, target) {
				r.Problems = append(r.Problems, fmt.Sprintf("%s: the link %s goes to no file", p, m[1]))
			}
		}
	}
	return r, nil
}

// internal returns the file path of a link to the site, relative to the root. It returns false for a link to another site, a fragment and a query.
func internal(page, link string) (string, bool) {
	link = strings.ReplaceAll(link, "&amp;", "&")
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Path == "" {
		return "", false
	}
	p := u.Path
	if !strings.HasPrefix(p, "/") {
		p = path.Join(path.Dir("/"+page), p)
		if strings.HasSuffix(u.Path, "/") {
			p += "/"
		}
	}
	return strings.TrimPrefix(p, "/"), true
}

func exists(files map[string]bool, p string) bool {
	if p == "" || strings.HasSuffix(p, "/") {
		return files[p+"index.html"]
	}
	return files[p] || files[p+"/index.html"]
}
