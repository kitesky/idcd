package format

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

type Formatter struct {
	format string
	out    io.Writer
}

func New(format string, out io.Writer) *Formatter {
	return &Formatter{format: format, out: out}
}

func (f *Formatter) IsJSON() bool {
	return f.format == "json"
}

func (f *Formatter) JSON(v any) error {
	enc := json.NewEncoder(f.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (f *Formatter) Table(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(f.out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

func (f *Formatter) Line(format string, args ...any) {
	fmt.Fprintf(f.out, format+"\n", args...)
}

func (f *Formatter) Error(err error) {
	fmt.Fprintf(f.out, "Error: %s\n", err.Error())
}
