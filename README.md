# figlet-font-preview

An interactive figlet font renderer: type a short text and see it rendered in every FIGlet and TOIlet font installed on your computer, with a fuzzy-searchable, multi-select font sidebar for comparing fonts side by side.

It renders fonts itself, so the `figlet` program does not need to be installed. Only the font files do.

## Install

```sh
go install github.com/jnaO/figlet-font-preview/cmd/figlet-font-preview@latest
```

Fonts are read from these directories when present:

- `/opt/homebrew/share/figlet`
- `/usr/local/share/figlet`
- `/usr/share/figlet`
- `~/figlet-fonts`

On macOS, `brew install figlet toilet` provides about 440 fonts. Fonts in different directories are listed separately even when they are the same font.

## Usage

```sh
figlet-font-preview                       # renders "Hello"
figlet-font-preview Hello world           # initial text
figlet-font-preview $'two\nlines'         # multiline initial text
figlet-font-preview --font-dir ~/fonts    # also search ~/fonts (repeatable)
figlet-font-preview --center Hello        # centre the text in the preview
```

### Which fonts are shown

| Search | Selection | Preview shows |
|---|---|---|
| empty | none | every font |
| empty | some | the selected fonts |
| active | any | the selected fonts, then every fuzzy match |

The selection is kept while you search and when you clear the search. The preview follows the highlighted font in the list, scrolling only when that font is not fully visible, and keeps it on the same row when you edit the text.

### Keys

| Where | Key | Action |
|---|---|---|
| text | type, `enter` | edit the text; `enter` starts a new line (the box grows to 5 rows, then scrolls) |
| anywhere | `tab` / `shift+tab` | cycle focus: text → search → list |
| anywhere | `pgup` / `pgdn`, `ctrl+u` / `ctrl+d`, mouse wheel | scroll the preview |
| anywhere | `ctrl+c` | quit |
| search | type | fuzzy-filter fonts |
| search | `↑` / `↓` | move in the list |
| search | `enter` | select or deselect the highlighted font |
| search | `esc` | clear the search |
| list | `↑` / `↓`, `j` / `k`, `g` / `G` | move |
| list | `space` / `enter` | select or deselect |
| list | `x` | clear the selection |
| list | `/` | jump to search |
| list | `esc` | clear the search |
| anywhere | `ctrl+o` | open or close the options pane |
| anywhere | `ctrl+y` | copy the cursor font's render to the clipboard as plain text, wrapped and centred as shown |
| list | `q` | quit |
| options | `↑` / `↓`, `j` / `k` | move |
| options | `space` / `enter` | toggle the row |
| options | `←` / `→` | change Group by |
| options | `tab` | back to the text, leaving the pane open |
| options | `esc` / `ctrl+o` | close the pane |

### Options panel

Press `ctrl+o` anywhere to slide the options pane in from the right. It has its own column, and changes apply immediately. `tab` moves between the text, search, list and pane.

- **Center text**: centres the text in the preview. `--center` sets the starting value.
- **Collapse duplicates**: shows fonts that are identical (the same font in two folders) only once, with `+N` showing how many copies are hidden.
- **Group by**: groups the font list by Stroke, Weight, Size or Slant. Press space on a group header to select or deselect the whole group.
- **Similar to**: lists the font under the list cursor, followed by its closest look-alikes, most similar first.
- **Filters**: every value of the four measured axes, with font counts. Within an axis, any checked value matches; across axes, all must match.
  - **Stroke**: what the letters are drawn with: line art (`/ \ | _`), solid fill (`# * @` or letters), block characters (`█`), box-drawing characters (`═ ║`), or other Unicode.
  - **Weight**: how much of the letter area is inked.
  - **Size**: how many rows tall the letters are.
  - **Slant**: upright or slanted.

Selected fonts always stay visible, whatever is filtered or collapsed. Categories are measured from how each font renders, not from its name.

## Development

```sh
go test ./...
```

The golden tests compare the renderer byte for byte with the real `figlet` for every installed font, and skip when `figlet` or the fonts are missing. See [ARCHITECTURE.md](ARCHITECTURE.md).

## License

[MIT](LICENSE). Font files are not part of this project and keep their own licenses.
