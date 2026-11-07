# figlet-display

An interactive figlet font renderer: type a short text and see it rendered in every FIGlet and TOIlet font installed on your computer, with a fuzzy-searchable, multi-select font sidebar for comparing fonts side by side.

It renders fonts itself, so the `figlet` program does not need to be installed. Only the font files do.

## Install

```sh
go install github.com/jnaO/figlet-display/cmd/figlet-display@latest
```

Fonts are read from these directories when present:

- `/opt/homebrew/share/figlet`
- `/usr/local/share/figlet`
- `/usr/share/figlet`
- `~/figlet-fonts`

On macOS, `brew install figlet toilet` provides about 440 fonts. Fonts in different directories are listed separately even when they are the same font.

## Usage

```sh
figlet-display                       # renders "Hello"
figlet-display Hello world           # initial text
figlet-display $'two\nlines'          # multiline initial text
figlet-display --font-dir ~/fonts    # also search ~/fonts (repeatable)
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
| list | `q` | quit |

## Development

```sh
go test ./...
```

The golden tests compare the renderer byte for byte with the real `figlet` for every installed font, and skip when `figlet` or the fonts are missing. See [ARCHITECTURE.md](ARCHITECTURE.md).
