// Command verify checks the static site: links, images, page basics, external
// URLs, identity leaks and HTML well-formedness. Standard library only.
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"html"
	"image/png"
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
	// Hosts a page may only link to (an <a href> or plain text), never load anything from.
	linkHosts = map[string]bool{
		"everloot.app": true, "claude.com": true, "code.claude.com": true, "platform.claude.com": true,
		"openai.com": true, "platform.openai.com": true, "openrouter.ai": true, "ollama.com": true,
		"lmstudio.ai": true, "antigravity.google": true,
	}
)

var problems []string

// navLinks holds every <a href> found on each page, to check the nav lists every page.
var navLinks = map[string][]string{}

// checkNav makes sure each page's links include every other real page (404 is not in the nav).
func checkNav(htmls []string) {
	for _, from := range htmls {
		have := map[string]bool{}
		for _, h := range navLinks[from] {
			have[strings.SplitN(h, "#", 2)[0]] = true
		}
		for _, to := range htmls {
			if to != "404.html" && !have[to] {
				fail(from, "nav does not list %s", to)
			}
		}
	}
}

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
	deny = append(deny, loadDeny(filepath.Join(root, "tools", "verify", "banned-invented.txt"))...)
	voice := loadDeny(filepath.Join(root, "tools", "verify", "banned-phrases.txt"))
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
			checkTexts(rel, s, deny)
			checkVoice(rel, s, voice)
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
	checkNav(htmls)
	checkShots(root, files)
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
		if allowedHosts[h] || (ext == ".html" && linkHosts[h]) {
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
			if name == "a" {
				if h, ok := at["href"]; ok {
					navLinks[f] = append(navLinks[f], h)
				}
			}
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
		} else if strings.HasPrefix(at["src"], shotsDir) {
			// a real screenshot: it is checked by checkShots, and need not say "example"
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
			checkRef(root, f, v, ids, name == "a")
		}
	}
}

func checkRef(root, f, v string, ids map[string]map[string]bool, anchor bool) {
	if v == "" {
		fail(f, "empty link")
		return
	}
	if strings.Contains(v, "://") {
		pu, err := url.Parse(v)
		if err != nil || !(allowedHosts[pu.Hostname()] || (anchor && linkHosts[pu.Hostname()])) || pu.Scheme != "https" {
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

const (
	shotsDir     = "assets/img/shots/"
	maxShotBytes = 400 * 1024
	maxShotSide  = 1400
)

var (
	altRe     = regexp.MustCompile(`(?is)\balt\s*=\s*"([^"]*)"`)
	captionRe = regexp.MustCompile(`(?is)<figcaption>(.*?)</figcaption>`)
	srcRe     = regexp.MustCompile(`(?is)<img\b[^>]*\bsrc\s*=\s*"([^"]*)"`)
	nonAlnum  = regexp.MustCompile(`[^a-z0-9]+`)
)

// checkTexts runs the identity check over alt text and captions alone, with
// punctuation and spacing stripped, so a name split by a hyphen, entity or space
// is still caught.
func checkTexts(f, s string, deny []string) {
	var texts []string
	for _, m := range altRe.FindAllStringSubmatch(s, -1) {
		texts = append(texts, m[1])
	}
	for _, m := range captionRe.FindAllStringSubmatch(s, -1) {
		texts = append(texts, m[1])
	}
	for _, t := range texts {
		if m := emailRe.FindString(t); m != "" {
			fail(f, "alt text or caption contains an email address: %q", m)
		}
		flat := nonAlnum.ReplaceAllString(strings.ToLower(t), "")
		for _, d := range deny {
			if k := nonAlnum.ReplaceAllString(d, ""); k != "" && strings.Contains(flat, k) {
				fail(f, "alt text or caption contains a denied name %q", d)
			}
		}
	}
}

// checkShots makes sure each file in the screenshots folder is used by a page,
// is under the size cap and has a longest side within the limit.
func checkShots(root string, files []string) {
	used := map[string]bool{}
	for _, f := range files {
		if path.Ext(f) != ".html" {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, m := range srcRe.FindAllStringSubmatch(string(b), -1) {
			used[m[1]] = true
		}
	}
	checkZoom(files)
	n := 0
	for _, f := range files {
		rel, _ := filepath.Rel(root, f)
		rel = filepath.ToSlash(rel)
		if !strings.HasPrefix(rel, shotsDir) {
			continue
		}
		n++
		if !used[rel] {
			fail(rel, "screenshot is not used by any page")
		}
		if st, err := os.Stat(f); err == nil && st.Size() > maxShotBytes {
			fail(rel, "screenshot is %d KB, over the %d KB cap", st.Size()/1024, maxShotBytes/1024)
		}
		w, h, err := imageSize(f)
		if err != nil {
			fail(rel, "cannot read image size: %v", err)
		} else if w > maxShotSide || h > maxShotSide {
			fail(rel, "screenshot is %dx%d, longest side must be at most %d", w, h, maxShotSide)
		}
	}
	if n == 0 {
		fail(shotsDir, "no screenshots found")
	}
}

// imageSize reads the pixel size of a PNG or a lossy or lossless WebP file.
func imageSize(p string) (int, int, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return 0, 0, err
	}
	switch path.Ext(p) {
	case ".png":
		c, err := png.DecodeConfig(strings.NewReader(string(b)))
		return c.Width, c.Height, err
	case ".webp":
		if len(b) < 30 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
			return 0, 0, fmt.Errorf("not a WebP file")
		}
		switch string(b[12:16]) {
		case "VP8 ":
			return int(binary.LittleEndian.Uint16(b[26:28]) & 0x3fff), int(binary.LittleEndian.Uint16(b[28:30]) & 0x3fff), nil
		case "VP8L":
			v := binary.LittleEndian.Uint32(b[21:25])
			return int(v&0x3fff) + 1, int(v>>14&0x3fff) + 1, nil
		case "VP8X":
			w := int(b[24]) | int(b[25])<<8 | int(b[26])<<16
			h := int(b[27]) | int(b[28])<<8 | int(b[29])<<16
			return w + 1, h + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("unsupported image type")
}

var (
	scriptRe   = regexp.MustCompile(`(?is)<(script|style)\b.*?</(script|style)>|<!--.*?-->`)
	voiceTagRe = regexp.MustCompile(`(?s)<[^>]*>`)
	attrTxtRe  = regexp.MustCompile(`(?is)\b(?:alt|content)\s*=\s*"([^"]*)"`)
	spaceRe    = regexp.MustCompile(`\s+`)
)

// checkVoice fails when a banned stock phrase appears in a page's visible text,
// alt text or meta description. Matching is case-insensitive and treats curly
// apostrophes as straight ones.
func checkVoice(f, s string, banned []string) {
	text := scriptRe.ReplaceAllString(s, " ")
	var parts []string
	for _, m := range attrTxtRe.FindAllStringSubmatch(text, -1) {
		parts = append(parts, m[1])
	}
	parts = append(parts, voiceTagRe.ReplaceAllString(text, " "))
	flat := strings.ToLower(html.UnescapeString(strings.Join(parts, " ")))
	flat = strings.NewReplacer("\u2019", "'", "\u2014", "—", "\u2013", "-").Replace(flat)
	flat = spaceRe.ReplaceAllString(flat, " ")
	for _, b := range banned {
		if strings.Contains(flat, b) {
			fail(f, "banned phrase in visible text: %q", b)
		}
	}
}

var (
	shotImgRe = regexp.MustCompile(`(?is)(<a\b[^>]*>\s*)?<img\b[^>]*\bsrc\s*=\s*"(assets/img/shots/[^"]*)"`)
	zoomRe    = regexp.MustCompile(`(?is)^<a\b[^>]*\bclass\s*=\s*"zoom"[^>]*\bhref\s*=\s*"([^"]*)"`)
)

// checkZoom makes sure every screenshot is wrapped in a lightbox link to its
// file, and that each page with one loads the lightbox script.
func checkZoom(files []string) {
	for _, f := range files {
		if path.Ext(f) != ".html" {
			continue
		}
		b, _ := os.ReadFile(f)
		s := string(b)
		found := false
		for _, m := range shotImgRe.FindAllStringSubmatch(s, -1) {
			found = true
			z := zoomRe.FindStringSubmatch(m[1])
			if z == nil || z[1] != m[2] {
				fail(f, "screenshot %s is not wrapped in a lightbox link (a.zoom)", m[2])
			}
		}
		if found && !strings.Contains(s, `src="assets/lightbox.js"`) {
			fail(f, "page has screenshots but does not load assets/lightbox.js")
		}
	}
}
