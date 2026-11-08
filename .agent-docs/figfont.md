# figfont — parsing and rendering FIGfonts

## Purpose

Parse FIGlet (`flf2a`) and TOIlet (`tlf2a`) fonts and render text **byte-identical to figlet 2.2.5**. The renderer is a deliberate port of figlet's C routines rather than a clean-room reading of the FIGfont spec, because figlet's behaviour diverges from the spec in places and figlet is the oracle.

## Mechanism

### Parsing (`font.go`, `reader.go`)

- **Zip**: data starting `PK\x03\x04` is unzipped and the first member parsed. Many installed `.tlf` fonts are zip archives with a single member named `-`.
- **UTF-8 BOM**: a leading `EF BB BF` is skipped (after unzipping). figlet rejects such files.
- **Signature**: the first 4 bytes are `flf2` or `tlf2`; `Font.Toilet` is set for `tlf2`.
- **Header**: `sscanf("%*c%c %d %*d %d %d %d %d %d")` — skips the `a`, then hardblank, height, (baseline ignored), max_length, old_layout, comment lines, print_direction, full_layout. Fewer than 5 values read → error.
- **Hardblank** is a single byte and may be `0x7F` (TOIlet) or `0xFF`.
- **Layout when full_layout is absent** (`parse`): old_layout `0` → kerning (`smKern`), `< 0` → full width, `> 0` → `(old & 31) | smSmush`.
- **Row decoding** (`Font.decode`): TOIlet rows are UTF-8 (invalid bytes dropped). FIGlet rows are UTF-8 when the row is valid UTF-8, otherwise one rune per byte (Latin-1). figlet treats FIGlet rows as bytes, so for UTF-8 `.flf` fonts (box-drawing/block fonts) it counts each multi-byte character as several columns when smushing and wrapping; we count one.
- **Endmarks** (`readGlyph`): strip trailing whitespace, then strip the whole run of the last remaining character. Not the spec's "one endmark, two on the last row".
- **Line reading** (`lineReader.fgets` = figlet's `myfgets`): keeps at most `max_length + 101` bytes per row and silently drops one byte when that limit is hit; a final row without a trailing newline reads as empty.
- **Glyph table**: 32–126, then the 7 Deutsch characters (196 214 220 228 246 252 223), then code-tagged characters parsed with C `strtol` base 0 (decimal, `0`-octal, `0x`-hex, negative). Later definitions override earlier ones. Missing characters fall back to ord 0, which is empty unless the font defines it.

### Rendering (`render.go`)

`Font.Render(text, width)` mirrors figlet's main loop and `addchar` / `smushamt` / `smushem` / `splitline` / `printline` / `putstring`, including the `wordbreakmode` state machine. `width <= 0` means `Unlimited`.

- **Width**: every output line is at most `width` runes. This equals `figlet -w width+1`.
- **Smushing state**: `prevW`/`currW` are updated on every `addChar` attempt, including failed ones during wrapping, exactly as figlet's `getletter`. Smushing is refused when either width is < 2.
- **C string semantics**: `at()` returns 0 past the end (the terminator); `cstr()` truncates at a 0 written by a failed smush; `in()` matches 0 like `strchr`.
- **RTL** (print_direction 1: `ivrit`, `jerusalem`, `mirror`, `mshebrew210`): characters prepend. Output is not justified; figlet right-justifies RTL fonts by default.
- Hardblanks become spaces only in `putString`, after smushing.
- **`Center(lines, height, width)`** pads each printed block (every `height` rows of `Render` output) by its widest row, so a block's art stays aligned. It is a separate step and not part of the figlet port.
- **`Trim(lines)`** strips trailing spaces, the indent shared by non-blank lines, and blank lines at both ends. Applied after `Center`, it keeps each block's offset relative to the widest block, which is what the TUI copies.

## Key Files

| File | Role |
|---|---|
| `internal/figfont/font.go` | header, zip, glyph table, endmark stripping |
| `internal/figfont/reader.go` | `myfgets`, header scanner, `strtol` |
| `internal/figfont/render.go` | layout, smushing rules, word wrap, RTL |
| `internal/figfont/golden_test.go` | oracle comparison for every installed font |
| `internal/figfont/testdata/` | portable fixtures: plain, zipped, 0x7F tlf, RTL |

## Gotchas

- **Golden tests pass `-l`** so figlet does not right-justify RTL fonts to the full `-w` width.
- **Malformed fonts can produce a negative smush amount** (glyph rows of uneven width). figlet then reads memory before its buffer; `smushAmount` clamps to 0 instead. Output next to such glyphs cannot match figlet.
- **`undefinedGlyphs` in `golden_test.go`** removes glyphs whose figlet output is undefined behaviour: `gradient` `}~` (file misaligned from `}` on) and `pyramid` space (0xFF hardblank, figlet prints uninitialised memory). Add an entry only after confirming figlet's own output is garbage, never to hide a renderer bug.
- **Intentional divergences from figlet**, all outside the golden sample: UTF-8 BOM skipped, UTF-8 `.flf` rows decoded as UTF-8, negative smush clamped.
- **Do not "fix" figlet quirks toward the spec** (endmark rule, old_layout 0 = kerning, the dropped byte in `fgets`). The golden test is the contract.
- `cursive.flf` is a symlink to `slscript.flf`, so real-path dedupe in `catalog` yields one font, named `slscript`.
