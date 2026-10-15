package ansible

import (
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxTasks int
	MaxHosts int
	MaxLines int
	All      bool
}

func DefaultOptions() Options {
	return Options{MaxTasks: 20, MaxHosts: 20, MaxLines: 20}
}

type Format struct{}

func (Format) Name() string { return "ansible" }

func (Format) Description() string {
	return "ansible-playbook runs from the default output or the ansible.posix.json callback"
}

var conditionNames = []string{"changed", "error", "fail", "unreachable", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

func (Format) Detect(head []byte) bool {
	return detectJSON(head) || detectText(head)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	fs.BoolVar(&o.All, "all", false, "also list changed tasks and the failures the run ignored or rescued")
	core.CountFlag(fs, &o.MaxHosts, "max-hosts", "recap lines of hosts without failures, 0 for all")
	core.CountFlag(fs, &o.MaxLines, "max-lines", "last lines of each msg, stderr or stdout, 0 for all")
	core.CountFlag(fs, &o.MaxTasks, "max-tasks", "task results listed, 0 for all")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	r, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	rep := Build(r, o)
	return core.Result{Text: Text(rep, o), Data: rep, Conditions: conditions(rep)}, nil
}

func conditions(r Report) []string {
	return core.Conditions(map[string]bool{
		"changed":     r.Changed > 0,
		"error":       r.Failed+r.Unreachable > 0 || len(r.Errors) > 0,
		"fail":        r.Failed+r.Unreachable > 0,
		"unreachable": r.Unreachable > 0,
		"warning":     len(r.Warnings) > 0,
	})
}
