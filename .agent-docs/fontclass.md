# fontclass — duplicates, facets and similarity

## Purpose

Profile every font by how it looks so front-ends can collapse exact duplicates, filter on measured facet axes, and list look-alikes. Everything is measured from fixed probe strings (`probe`, `slantGlyphs`, the ASCII run), never from the user's text, so a font's profile never changes while typing. Deterministic: same fonts, same profiles, same order.

## Mechanism

### Duplicates

- **Key** is `figfont.Font.GlyphKey()`: SHA-256 over height, layout, print direction and the rows of 32–126 plus the Deutsch characters, with the hardblank normalised. It lives in `figfont` because the glyph table is private. The space glyph is part of the key (its width changes typed text).
- **Representative** (`Index`, built by `New`): the group member with the lowest `catalog.Entry.Root`, then the first by name. `Duplicates(rep)` is the `+N` count; non-representatives return 0.
- Same-name fonts that differ (e.g. different layout) are not duplicates; they come out as near neighbours in `Similar`.

### Facet axes (`Analyze`)

Measured on `probe` rendered at unlimited width with blank rows trimmed.

| Axis | Measure | Rule |
|---|---|---|
| Stroke | share of ink per class: block U+2580–259F, box U+2500–257F, other runes > 126, `lineChars`, everything else = fill | a non-ASCII class with ≥ `nonASCIIShare` of ink wins (largest first); otherwise Line art vs Solid fill by majority |
| Weight | ink cells / probe bounding box | Light < `lightBelow`, Heavy ≥ `heavyFrom` |
| Size | ink rows | `sizeUpTo` buckets |
| Slant | `slantScore` | Slanted ≥ `slantedFrom` |

`Axes()` is THE list of axes and values in display order; front-ends never hard-code them. `Profile.Values` indexes into it.

### Slant

`slantScore = min(edgeSlant(keep horizontals), edgeSlant(skip horizontals))`. `edgeSlant` regresses the left and right ink edge on the row for each of `slantGlyphs`, averages the two slopes, and takes the median over glyphs with ≥ `minSlantRows` measured rows. Rows drawn only with `horizontals` never count.

Why each piece exists (all found on real fonts):
- Both edges and the median: left-edge-only made `standard` and serif fonts read slanted.
- Skipping horizontal-only rows: a top bar of `_` sits beside the vertical stroke below it (`mini`, `straight`).
- The min of the two variants: counting horizontals, a drop shadow's `_` reads as a lean (`smshadow`); skipping them, a bar row's far `|` becomes the edge (`shadow`). Real slants score the same either way.
- `minSlantRows = 2`: `italic` has 2-row glyphs once its top bar is skipped.

Known weak spots: 2–3 row fonts (e.g. `mike`) can score anything; bitmap italics sit just above the threshold.

### Similarity

`Profile.Fingerprint` = stroke-class shares ×3, density ×2, log ink rows, log(width per probe glyph / rows), slant, and a 6×32 ink-density grid ×0.5. `Index.Similar(name)` orders every other profiled font by Euclidean distance, ties by name; exact duplicates come first at distance 0.

## Key Files

| File | Role |
|---|---|
| `internal/fontclass/analyze.go` | `Analyze`, axes, thresholds, slant, fingerprint |
| `internal/fontclass/index.go` | `Index`: duplicates, representatives, values, `Similar` |
| `internal/fontclass/fontclass_test.go` | portable boundary tests; installed calibration table, duplicate groups and neighbour assertions |
| `internal/figfont/font.go` | `Font.GlyphKey` |

## Gotchas

- **The calibration table in `fontclass_test.go` is the contract.** Every label was checked by viewing the font. Change a threshold only if the table still passes, and add a label only after viewing the font.
- Installed tests use `catalog.SystemRoots` only; never tune on `~/figlet-fonts`, which tests cannot see.
- Fonts that fail to parse have no profile: they belong to no duplicate group and never appear in `Similar`.
