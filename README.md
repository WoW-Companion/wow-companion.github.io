# wow-companion.github.io

The player website for WoW Companion, served at https://wow-companion.github.io.

It is plain static HTML, CSS, a little vanilla JavaScript and SVG. There is no framework, no build step and
no third-party scripts, fonts or trackers. Every merge to `main` is published by GitHub Pages.

All logos and illustrations are original work made for this site. They are mockups with invented names, not
screenshots.

## Preview locally

Serve this folder with any static file server and open the address it prints. For example, with Go installed:

```
go run ./tools/serve
```

Or use whichever server you already have (for example `npx serve .`). Opening `index.html` straight from disk
also works for a quick look.

## Check your changes

```
make verify
```

This needs Go. It checks that every internal link and image resolves, every page has a title, a description,
alt text and the footer disclaimer, no external scripts, fonts or stylesheets are used, no real names or paths
slipped in, and the HTML is well-formed. To add a name that must never appear, put it in
`tools/verify/deny-names.txt`.

## Layout

- `index.html`, `getting-started.html`, `404.html` and the other pages at the top level
- `assets/site.css`, `assets/logo.svg`, `assets/img/*.svg`
- `tools/verify`: the checker

## Rules

The footer disclaimer goes on every page. Claims about the app must match the app's own guides. Never use
Blizzard's logos, art, fonts or screenshots.
