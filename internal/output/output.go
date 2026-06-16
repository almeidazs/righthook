package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/almeidazs/righthook/internal/git"
	"github.com/orochaa/go-clack/third_party/picocolors"
)

type Renderer struct {
	out    io.Writer
	symbol symbols
	json   bool
}

type symbols struct {
	Section string
	Block   string
	Bar     string
	Branch  string
	Last    string
	OK      string
	Warn    string
	Err     string
	Glove   string
}

type Result struct {
	Warnings   []string        `json:"warnings,omitempty"`
	Detection  any             `json:"detection,omitempty"`
	Options    any             `json:"options,omitempty"`
	Plan       git.InstallPlan `json:"plan"`
	Selected   any             `json:"selected,omitempty"`
	Validated  bool            `json:"validated"`
	Validation string          `json:"validation,omitempty"`
}

func New(out io.Writer, noEmoji, asJSON bool) Renderer {
	s := symbols{
		Section: "◇",
		Block:   "◆",
		Bar:     "│",
		Branch:  "├─",
		Last:    "└─",
		OK:      "✓",
		Warn:    "▲",
		Err:     "✕",
		Glove:   "🥊",
	}
	if noEmoji {
		s = symbols{
			Section: ">",
			Block:   "*",
			Bar:     "|",
			Branch:  "|-",
			Last:    "`-",
			OK:      "ok",
			Warn:    "!!",
			Err:     "x",
			Glove:   "[countered]",
		}
	}
	return Renderer{out: out, symbol: s, json: asJSON}
}

func (r Renderer) Intro(title string) {
	if r.json {
		return
	}
	fmt.Fprintf(r.out, "\n%s %s\n", picocolors.Cyan(r.symbol.Section), title)
}

func (r Renderer) Section(title string) {
	if r.json {
		return
	}
	fmt.Fprintf(r.out, "\n%s %s\n", picocolors.Green(r.symbol.Block), title)
}

func (r Renderer) Item(label, value string) {
	if r.json {
		return
	}
	fmt.Fprintf(r.out, "%s %s: %s\n", picocolors.Gray(r.symbol.Bar), label, value)
}

func (r Renderer) List(title string, items []string) {
	if r.json || len(items) == 0 {
		return
	}
	fmt.Fprintf(r.out, "%s %s\n", picocolors.Green(r.symbol.Block), title)
	for i, item := range items {
		prefix := r.symbol.Branch
		if i == len(items)-1 {
			prefix = r.symbol.Last
		}
		fmt.Fprintf(r.out, "%s %s\n", picocolors.Gray(prefix), item)
	}
}

func (r Renderer) Warn(msg string) {
	if r.json {
		return
	}
	fmt.Fprintf(r.out, "%s %s\n", picocolors.Yellow(r.symbol.Warn), msg)
}

func (r Renderer) Success(msg string) {
	if r.json {
		return
	}
	fmt.Fprintf(r.out, "%s %s\n", picocolors.Green(r.symbol.OK), msg)
}

func (r Renderer) Spacer() {
	if r.json {
		return
	}
	fmt.Fprintln(r.out)
}

func (r Renderer) Countered() {
	if r.json {
		return
	}
	fmt.Fprintf(r.out, "\n%s %s Lefthook got countered.\n", picocolors.Green(r.symbol.OK), r.symbol.Glove)
}

func (r Renderer) Error(msg string) {
	if r.json {
		return
	}
	fmt.Fprintf(r.out, "%s %s\n", picocolors.Red(r.symbol.Err), msg)
}

func (r Renderer) DryRun(plan git.InstallPlan, hookNames []string, jobs map[string][]string) {
	if r.json {
		return
	}
	r.Section("Dry run")
	r.Item("config", plan.ConfigPath)
	if len(hookNames) > 0 {
		r.List("hooks", hookNames)
	}
	var lines []string
	for _, hook := range hookNames {
		lines = append(lines, fmt.Sprintf("%s: %s", hook, strings.Join(jobs[hook], ", ")))
	}
	r.List("jobs", lines)
	if len(plan.GitIgnoreAdditions) > 0 {
		r.List("gitignore", plan.GitIgnoreAdditions)
	}
}

func (r Renderer) JSON(result Result) error {
	enc := json.NewEncoder(r.out)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
