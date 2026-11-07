# Architecture

Planning lives in GitHub issues on `jnaO/figlet-display`: Stories carry the plan, Task sub-issues track the steps.

`figlet-display` is a Go terminal app that renders a sample string in every installed FIGlet/TOIlet font, with a fuzzy-searchable, multi-select font sidebar. It is standalone: it reads font files from disk and never runs the `figlet` binary, which is only the test oracle.

## Dependency direction

```
cmd/figlet-display → tui → display
                       ├──→ catalog → figfont
                       └──→ figfont (Center)
```

Nothing below `internal/tui` imports Bubble Tea, bubbles or lipgloss (check: `go list -deps ./internal/figfont ./internal/catalog ./internal/display | grep charmbracelet` prints nothing). A second front-end (e.g. a desktop GUI) drives `display.State` and `catalog.Catalog` directly.

## Map

```
cmd/figlet-display/main.go   flags (--font-dir repeatable, --center, --version), positional text (default "Hello"); roots = catalog.DefaultRoots() + --font-dir
internal/figfont/
  font.go          Parse/ParseFile: flf2a + tlf2a headers, zip-archived fonts, glyph table; THE font parser
  reader.go        byte-level line reader reproducing figlet's myfgets, header sscanf subset, C strtol (code tags)
  render.go        Font.Render(text, width): THE renderer — port of figlet's layout, smushing, RTL and word wrap
  golden_test.go   every installed font vs the figlet binary, unwrapped and wrapped; skips without figlet/fonts
  font_test.go     portable fixture tests (testdata/: tiny flf, zipped flf, 0x7F-hardblank tlf, RTL flf)
internal/catalog/catalog.go
                   SystemRoots (package-manager dirs; the only roots tests use), DefaultRoots() = SystemRoots + ~/figlet-fonts;
                   Discover(roots): recursive walk following symlinks, *.flf/*.tlf only, dedupe by real path,
                   unique names; Entry.Load (once, concurrent-safe), Entry.State/Font (non-blocking), Preload(workers)
internal/display/display.go
                   State: text, Center, query, selection; Sidebar() and RenderSet(); THE render-set rule
internal/tui/
  tui.go           Bubble Tea model: multiline text area (grows to maxTextLines rows), sidebar (search + list),
                   preview viewport, footer help
  lolcat.go        rainbow colouring of the cursor font (lolcat -S 20 -p 7 -F 0.05 equivalent) on the tint background
  styles.go        all lipgloss styles (adaptive light/dark)
```

## Patterns

- **Render set** (`display.State.RenderSet`): no query + no selection → all fonts; no query → selection (sorted); query → selection first, then fuzzy matches not already listed. The sidebar shows all fonts without a query, otherwise only matches in `sahilm/fuzzy` rank order.
- **Font names** (`catalog.uniqueNames`): basename without extension; on a basename collision, `parentdir/basename`; on a further collision, the full path minus extension. Names come from the resolved real path, so a symlink alias collapses into its target's name.
- **Cursor font in the preview** (`tui.Model.revealCursor`, `revealOffset`): `previewContent` records each rendered font's line span; on cursor move, query change or toggle the preview scrolls the minimum needed to show the cursor font in full (its top when taller than the view), and never when it is already fully visible or not in the render set. The cursor font is drawn with `lolcat`.
- **Preview anchor** (`tui.Model.refreshPreview`, `anchorFont`): every re-render that is not a reset keeps the anchor font (the cursor font if rendered, else the font at the top of the view) on the same screen row, so text edits, resizes and fonts finishing loading do not shift it.
- **Text area sizing** (`tui.Model.handleKey`, `resetTextScroll`): the bubbles textarea scrolls its own view during `Update` from its current height, so it is updated at `maxTextLines` and then fitted; after lines are deleted its stale scroll offset is reset via `SetValue` with the cursor restored. Tests call `View()` after each update because bubbles components only learn their content size when rendered.
- **Lazy loading**: the TUI calls `Catalog.Preload` at start and polls on a tick until the returned channel closes; preview entries show `loading…` or the parse error until ready. The full preview is re-rendered on every change.

## Drill-downs

- [.agent-docs/figfont.md](.agent-docs/figfont.md) — font format quirks, figlet-compatibility rules, golden-test exclusions.
