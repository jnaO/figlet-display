// Command figlet-display renders text in every installed FIGlet/TOIlet font
// in an interactive terminal UI.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jnaO/figlet-display/internal/catalog"
	"github.com/jnaO/figlet-display/internal/tui"
)

var version = "dev"

type dirList []string

func (d *dirList) String() string     { return strings.Join(*d, ",") }
func (d *dirList) Set(v string) error { *d = append(*d, v); return nil }

func main() {
	var dirs dirList
	flag.Var(&dirs, "font-dir", "extra directory to search for fonts (repeatable)")
	center := flag.Bool("center", false, "centre the rendered text in the preview")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: figlet-display [--center] [--font-dir DIR]... [text]\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nDefault font roots: %s\n", strings.Join(catalog.DefaultRoots(), ", "))
	}
	flag.Parse()
	if *showVersion {
		fmt.Println("figlet-display", version)
		return
	}

	text := "Hello"
	if flag.NArg() > 0 {
		text = strings.Join(flag.Args(), " ")
	}

	cat := catalog.Discover(append(catalog.DefaultRoots(), dirs...))
	p := tea.NewProgram(tui.New(cat, text, *center), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
