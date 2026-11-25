// Command verify checks the static site: links, images, page basics, external
// URLs, identity leaks and HTML well-formedness. Standard library only.
package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const disclaimer = "WoW Companion is a free, open-source fan project. It is not affiliated with or endorsed by Blizzard Entertainment. World of Warcraft is a trademark of Blizzard Entertainment, Inc."

var (
	tagRe     = regexp.MustCompile(`(?s)<(/?)([a-zA-Z][a-zA-Z0-9]*)((?:"[^"]*"|'[^']*'|[^>"'])*)>`)
	attrRe    = regexp.MustCompile(`([a-zA-Z:-]+)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	commentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
	rawRe     = regexp.MustCompile(`(?s)(<(?:script|style)\b[^>]*>).*?(</(?:script|style)>)`)
	urlRe     = regexp.MustCompile(`https?://[^\s"'<>)]+`)
	emailRe   = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	pathRe    = regexp.MustCompile(`(?i)(/Users/|/home/[a-z]|[A-Z]:\\Users\\|\\Users\\)`)
	idRe      = regexp.MustCompile(`\bid\s*=\s*["']([^"']+)["']`)
	titleRe   = regexp.MustCompile(`(?s)<title>\s*\S.*?</title>`)
	descRe    = regexp.MustCompile(`<meta\s+name="description"\s+content="[^"]{20,}"`)
	void      = map[string]bool{"meta": true, "link": true, "img": true, "br": true, "hr": true, "input": true, "base": true, "source": true, "wbr": true}
	// Hosts any file may point at: links, plus the one release API call.
	allowedHosts = map[string]bool{"github.com": true, "api.github.com": true, "wow-companion.github.io": true}
)

var problems []string

func fail(f, format string, a ...any) { problems = append(problems, f+": "+fmt.Sprintf(format, a...)) }

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	var files []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, p)
		return nil
	})
	deny := loadDeny(filepath.Join(root, "tools", "verify", "deny-names.txt"))
	ids := map[string]map[string]bool{}
	var htmls []string
	for _, f := range files {
		rel, _ := filepath.Rel(root, f)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "tools/") || rel == "Makefile" || rel == "go.mod" {
			continue // the checker's own source is not site content
		}
		b, _ := os.ReadFile(f)
		s := string(b)
		ext := path.Ext(rel)
		switch ext {
		case ".html":
			htmls = append(htmls, rel)
			ids[rel] = map[string]bool{}
			for _, m := range idRe.FindAllStringSubmatch(s, -1) {
				ids[rel][m[1]] = true
			}
		case ".svg":
			checkXML(rel, s)
		}
		switch ext {
		case ".html", ".css", ".js", ".svg", ".md", ".txt", ".json", ".xml":
			checkIdentity(rel, s, deny)
			checkURLs(rel, s, ext)
		}
	}
	for _, h := range htmls {
		b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(h)))
		checkPage(root, h, string(b), ids)
	}
	if _, err := os.Stat(filepath.Join(root, ".nojekyll")); err != nil {
		fail(".nojekyll", "missing")
	}
	if len(htmls) == 0 {
		fail("site", "no HTML pages found")
	}
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Println("FAIL", p)
		}
		fmt.Printf("verify: %d problem(s)\n", len(problems))
		os.Exit(1)
	}
	fmt.Printf("verify: ok (%d pages, %d files)\n", len(htmls), len(files))
}

func loadDeny(p string) []string {
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, strings.ToLower(l))
		}
	}
	return out
}

func checkIdentity(f, s string, deny []string) {
	if m := emailRe.FindString(s); m != "" {
		fail(f, "contains an email address: %q", m)
	}
	if m := pathRe.FindString(s); m != "" {
		fail(f, "contains a user path (%q)", m)
	}
	low := strings.ToLower(s)
	for _, d := range deny {
		if strings.Contains(low, d) {
			fail(f, "contains a denied name %q", d)
		}
	}
}

func checkURLs(f, s, ext string) {
	for _, u := range urlRe.FindAllString(s, -1) {
		pu, err := url.Parse(strings.TrimRight(u, ".,;"))
		if err != nil {
			fail(f, "bad URL %q", u)
			continue
		}
		h := pu.Hostname()
		if allowedHosts[h] {
			continue
		}
		if ext == ".svg" && h == "www.w3.org" && strings.HasSuffix(pu.Path, "/2000/svg") {
			continue // the SVG namespace, not a request
		}
		fail(f, "external URL not allowed: %s", u)
	}
}

func checkXML(f, s string) {
	d := xml.NewDecoder(strings.NewReader(s))
	for {
		_, err := d.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			fail(f, "not well-formed SVG: %v", err)
			return
		}
	}
}

func checkPage(root, f, s string, ids map[string]map[string]bool) {
	clean := commentRe.ReplaceAllString(s, "")
	if !strings.Contains(clean, disclaimer) {
		fail(f, "footer disclaimer missing")
	}
	if !titleRe.MatchString(clean) {
		fail(f, "missing or empty <title>")
	}
	if !descRe.MatchString(clean) {
		fail(f, "missing description meta tag")
	}
	if !strings.Contains(clean, "<!DOCTYPE html>") {
		fail(f, "missing doctype")
	}
	body := rawRe.ReplaceAllString(clean, "$1$2") // script and style bodies are not markup
	var stack []string
	for _, m := range tagRe.FindAllStringSubmatch(body, -1) {
		closing, name, attrs := m[1] == "/", strings.ToLower(m[2]), m[3]
		if !closing {
			at := map[string]string{}
			for _, a := range attrRe.FindAllStringSubmatch(attrs, -1) {
				v := a[2]
				if a[3] != "" {
					v = a[3]
				}
				at[strings.ToLower(a[1])] = v
			}
			checkTag(root, f, name, at, ids)
			if !void[name] && !strings.HasSuffix(strings.TrimSpace(attrs), "/") {
				stack = append(stack, name)
			}
			continue
		}
		if void[name] {
			continue
		}
		if len(stack) == 0 || stack[len(stack)-1] != name {
			fail(f, "unbalanced </%s> (open: %v)", name, tail(stack))
			return
		}
		stack = stack[:len(stack)-1]
	}
	if len(stack) > 0 {
		fail(f, "unclosed tags: %v", stack)
	}
	seen := map[string]bool{}
	for _, m := range idRe.FindAllStringSubmatch(clean, -1) {
		if seen[m[1]] {
			fail(f, "duplicate id %q", m[1])
		}
		seen[m[1]] = true
	}
}

func tail(s []string) []string {
	if len(s) > 3 {
		return s[len(s)-3:]
	}
	return s
}

func checkTag(root, f, name string, at map[string]string, ids map[string]map[string]bool) {
	switch name {
	case "img":
		alt := strings.TrimSpace(at["alt"])
		if alt == "" {
			fail(f, "img %q has no alt text", at["src"])
		} else if !strings.Contains(at["src"], "logo") && !strings.Contains(strings.ToLower(alt), "example") {
			fail(f, "illustration %q: alt text must say it is an example", at["src"])
		}
	case "script":
		if src := at["src"]; strings.Contains(src, "://") || strings.HasPrefix(src, "//") {
			fail(f, "external script %q", src)
		}
	case "link":
		if h := at["href"]; strings.Contains(h, "://") || strings.HasPrefix(h, "//") {
			fail(f, "external <link> %q", h)
		}
	}
	if name == "base" {
		return
	}
	for _, k := range []string{"href", "src"} {
		if v, ok := at[k]; ok {
			checkRef(root, f, v, ids)
		}
	}
}

func checkRef(root, f, v string, ids map[string]map[string]bool) {
	if v == "" {
		fail(f, "empty link")
		return
	}
	if strings.Contains(v, "://") {
		pu, err := url.Parse(v)
		if err != nil || !allowedHosts[pu.Hostname()] || pu.Scheme != "https" {
			fail(f, "link to a non-GitHub or non-https URL: %s", v)
		}
		return
	}
	if strings.HasPrefix(v, "mailto:") || strings.HasPrefix(v, "tel:") || strings.HasPrefix(v, "//") {
		fail(f, "disallowed link %q", v)
		return
	}
	pu, err := url.Parse(v)
	if err != nil {
		fail(f, "bad link %q", v)
		return
	}
	target := f
	if pu.Path != "" {
		if strings.HasPrefix(pu.Path, "/") {
			target = strings.TrimPrefix(pu.Path, "/")
		} else {
			target = path.Join(path.Dir(f), pu.Path)
		}
	}
	if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(target))); err != nil || st.IsDir() {
		fail(f, "link target not found: %s", v)
		return
	}
	if pu.Fragment != "" && path.Ext(target) == ".html" && !ids[target][pu.Fragment] {
		fail(f, "no #%s in %s", pu.Fragment, target)
	}
}
