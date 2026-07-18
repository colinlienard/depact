package cli

import (
	"fmt"
	"io"
	"os"
)

type style struct{ on bool }

func styleFor(w io.Writer) style {
	if os.Getenv("NO_COLOR") != "" {
		return style{}
	}
	f, ok := w.(*os.File)
	if !ok {
		return style{}
	}
	info, err := f.Stat()
	if err != nil {
		return style{}
	}
	return style{on: info.Mode()&os.ModeCharDevice != 0}
}

func (s style) wrap(code, v string) string {
	if !s.on {
		return v
	}
	return "\x1b[" + code + "m" + v + "\x1b[0m"
}

func (s style) bold(v string) string { return s.wrap("1", v) }
func (s style) dim(v string) string  { return s.wrap("2", v) }
func (s style) cyan(v string) string { return s.wrap("36", v) }
func (s style) red(v string) string  { return s.wrap("31", v) }

func (s style) num(n int) string { return s.cyan(fmt.Sprintf("%d", n)) }
