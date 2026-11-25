# wow-companion.github.io

The player website for WoW Companion, served at https://wow-companion.github.io.

It is plain static HTML, CSS, a little vanilla JavaScript and SVG. There is no framework, no build step and
no third-party scripts, fonts or trackers. Every merge to `main` is published by GitHub Pages.

Logos and the drawings in `assets/img/*.svg`, `assets/img/shots/*` are original work made for this site (mockups with invented
names). The photos in `assets/img/shots/` are the project's own screenshots from a real play session, with character
names pixelated. Keep them that way.

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
slipped in, every page's navigation lists every other page, and the HTML is well-formed. Pages may link to a short list of provider sites (see `linkHosts` in `tools/verify/main.go`) but never load anything from them. To add a name that must never appear, put it in
`tools/verify/deny-names.txt`.

Invented gameplay names that must never return (places and items that do not exist in the game) are listed in
`tools/verify/banned-invented.txt`; `make verify` fails if one appears on any page.

## Layout

- `index.html`, `getting-started.html`, `404.html` and the other pages at the top level
- `assets/site.css`, `assets/logo.svg`, `assets/img/*.svg`
- `tools/verify`: the checker

## Rules

The footer disclaimer goes on every page. Claims about the app must match the app's own guides. Never use
Blizzard's logos, art or fonts. The only game screenshots allowed are the project's own in `assets/img/shots/`.

**Fact-check rule.** Every gameplay claim or example on the site (quests, NPCs, zones, items, recipes, skill
levels, costs, advice) must be checked against the WoW Companion dataset or a cited reliable guide before it
goes in. Nothing is invented. Examples use an invented character, but the quests, items and steps they name
are real. Record each example's checks in the change's report or pull request.
