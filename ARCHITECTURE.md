# Architecture

Planning lives in GitHub issues on `jnaO/figlet-font-preview`: Stories carry the plan, Task sub-issues track the steps.

`figlet-font-preview` is a Go terminal app that renders a sample string in every installed FIGlet/TOIlet font, with a fuzzy-searchable, multi-select font sidebar. It is standalone: it reads font files from disk and never runs the `figlet` binary, which is only the test oracle.

## Dependency direction

```
cmd/figlet-font-preview → tui → display → fontclass → catalog → figfont
                       ├──→ catalog
                       └──→ figfont (Center)
```

Nothing below `internal/tui` imports Bubble Tea, bubbles or lipgloss (check: `go list -deps ./internal/figfont ./internal/catalog ./internal/display ./internal/fontclass | grep charmbracelet` prints nothing). A second front-end (e.g. a desktop GUI) drives `display.State` and `catalog.Catalog` directly.

## Map

```
cmd/figlet-font-preview/main.go   flags (--font-dir repeatable, --center, --version), positional text (default "Hello"); roots = catalog.DefaultRoots() + --font-dir
internal/figfont/
  font.go          Parse/ParseFile: flf2a + tlf2a headers, zip-archived fonts, glyph table; THE font parser
  reader.go        byte-level line reader reproducing figlet's myfgets, header sscanf subset, C strtol (code tags)
  font.go/GlyphKey exact-duplicate key (glyph table is private to figfont)
  render.go        Font.Render(text, width): THE renderer — port of figlet's layout, smushing, RTL and word wrap
  golden_test.go   every installed font vs the figlet binary, unwrapped and wrapped; skips without figlet/fonts
  font_test.go     portable fixture tests (testdata/: tiny flf, zipped flf, 0x7F-hardblank tlf, RTL flf)
internal/catalog/catalog.go
                   SystemRoots (package-manager dirs; the only roots tests use), DefaultRoots() = SystemRoots + ~/figlet-fonts;
                   Discover(roots): recursive walk following symlinks, *.flf/*.tlf only, dedupe by real path
                   (Entry.Root = index of the first root that reached the file),
                   unique names; Entry.Load (once, concurrent-safe), Entry.State/Font (non-blocking), Preload(workers)
internal/fontclass/
  analyze.go       Analyze(font) Profile: Stroke/Weight/Size/Slant from fixed probes; Axes() is THE axis list
  index.go         Index: duplicate groups + representative, facet values, Similar(name) by fingerprint
internal/display/display.go
                   State: text, Center, query, selection; Sidebar() and RenderSet(); THE render-set rule
internal/tui/
  tui.go           Bubble Tea model: multiline text area (grows to maxTextLines rows), sidebar (search + list),
                   preview viewport, footer help
  options.go       options pane, a right-hand column toggled by global ctrl+o (toggleOptions; slides in/out via
                   slideMsg, panelW feeds previewWidth): optionRows (Center, Collapse, Group by, Similar to the
                   cursor font, one row per facet value, Clear filters), applyOption, itemsChanged (keeps the
                   cursor font), buildIndex/indexMsg
  lolcat.go        rainbow colouring of the cursor font (lolcat -S 20 -p 7 -F 0.05 equivalent) on the tint background
  styles.go        all lipgloss styles (adaptive light/dark)
```

## Patterns

- **Universe pipeline** (`display.State.universe`): with a `fontclass.Index` set, the font list first goes through Collapse (representatives only), then facet filters (OR within an axis, AND across axes), then the Similar anchor plus up to `SimilarCap` look-alikes in distance order; selected fonts are always added back. With no index or every option off, the universe is all fonts.
- **Render set** (`display.State.RenderSet`), over the universe: no query + no selection → the sidebar's fonts in sidebar order; no query → selection (sorted); query → selection first, then fuzzy matches not already listed. The sidebar shows the universe without a query, otherwise only matches in `sahilm/fuzzy` rank order; with `GroupBy` set, a header `Item` precedes each non-empty group (axis value order, `Unclassified` last) and `ToggleGroup` selects or deselects the whole group.
- **Font names** (`catalog.uniqueNames`): basename without extension; on a basename collision, `parentdir/basename`; on a further collision, the full path minus extension. Names come from the resolved real path, so a symlink alias collapses into its target's name.
- **Cursor font in the preview** (`tui.Model.revealCursor`, `revealOffset`): `previewContent` records each rendered font's line span; on cursor move, query change or toggle the preview scrolls the minimum needed to show the cursor font in full (its top when taller than the view), and never when it is already fully visible or not in the render set. The cursor font is drawn with `lolcat`.
- **Preview anchor** (`tui.Model.refreshPreview`, `anchorFont`): every re-render that is not a reset keeps the anchor font (the cursor font if rendered, else the font at the top of the view) on the same screen row, so text edits, resizes and fonts finishing loading do not shift it.
- **Text area sizing** (`tui.Model.handleKey`, `resetTextScroll`): the bubbles textarea scrolls its own view during `Update` from its current height, so it is updated at `maxTextLines` and then fitted; after lines are deleted its stale scroll offset is reset via `SetValue` with the cursor restored. Tests call `View()` after each update because bubbles components only learn their content size when rendered.
- **Lazy loading**: the TUI calls `Catalog.Preload` at start and polls on a tick until the returned channel closes; preview entries show `loading…` or the parse error until ready. When loading finishes, `buildIndex` builds the `fontclass.Index` off the UI goroutine and `indexMsg` hands it to `display.State.SetIndex`; until then the options panel's index-backed rows show `analysing…` and do nothing. The full preview is re-rendered on every change.
- **Sidebar headers** (`display.Item.IsHeader`): a header has no `Name`, so `cursorName()` is empty on it and the preview reveal/anchor ignore it; space on a header calls `ToggleGroup`.

## Drill-downs

- [.agent-docs/figfont.md](.agent-docs/figfont.md) — font format quirks, figlet-compatibility rules, golden-test exclusions.
- [.agent-docs/fontclass.md](.agent-docs/fontclass.md) — duplicate key, facet measures and thresholds, slant calibration, fingerprint.
