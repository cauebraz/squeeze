package core

import (
	"fmt"
	"strings"
)

type Block struct {
	Lines   []string `json:"lines"`
	Omitted int      `json:"lines_omitted"`
}

func HeadBlock(lines []string, limit int) Block {
	kept, omitted := Limit(lines, limit)
	return Block{Lines: append([]string{}, kept...), Omitted: omitted}
}

func TailBlock(lines []string, limit int) Block {
	kept, omitted := LimitTail(lines, limit)
	return Block{Lines: append([]string{}, kept...), Omitted: omitted}
}

func (b Block) Write(w *strings.Builder, indent, flag string) {
	b.writeLines(w, indent)
	if b.Omitted > 0 {
		fmt.Fprintf(w, "%s... %d more %s (%s 0)\n", indent, b.Omitted, Plural(b.Omitted, "line", "lines"), flag)
	}
}

func (b Block) WriteTail(w *strings.Builder, indent, flag string) {
	if b.Omitted > 0 {
		fmt.Fprintf(w, "%s... %d earlier %s (%s 0)\n", indent, b.Omitted, Plural(b.Omitted, "line", "lines"), flag)
	}
	b.writeLines(w, indent)
}

func (b Block) writeLines(w *strings.Builder, indent string) {
	for _, l := range b.Lines {
		w.WriteString(indent)
		w.WriteString(l)
		w.WriteByte('\n')
	}
}
