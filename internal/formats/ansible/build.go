package ansible

type Task struct {
	Status Status `json:"status"`
	Host   string `json:"host"`
	Play   string `json:"play"`
	Task   string `json:"task"`
	Path   string `json:"path"`
	Item   string `json:"item,omitempty"`
	Detail
}

type Report struct {
	Tool         string      `json:"tool"`
	ToolVersion  string      `json:"tool_version"`
	Input        string      `json:"input"`
	Plays        int         `json:"plays"`
	Recap        bool        `json:"recap"`
	Hosts        []HostRecap `json:"hosts"`
	HostsOmitted int         `json:"hosts_omitted"`
	Failed       int         `json:"failed"`
	Unreachable  int         `json:"unreachable"`
	Changed      int         `json:"changed"`
	Ignored      int         `json:"ignored"`
	Rescued      int         `json:"rescued"`
	Recovered    int         `json:"recovered"`
	Tasks        []Task      `json:"tasks"`
	TasksOmitted int         `json:"tasks_omitted"`
	Errors       []string    `json:"errors"`
	Warnings     []string    `json:"warnings"`
}

type key struct {
	host string
	seq  int
}

func classify(r Run) []Event {
	events := append([]Event(nil), r.Events...)
	recap := map[string]HostRecap{}
	for _, h := range r.Recap {
		recap[h.Host] = h
	}
	groups := map[string][]int{}
	for i := range events {
		e := &events[i]
		if e.Status != StatusFailed && e.Status != StatusUnreachable {
			continue
		}
		if e.Ignored {
			e.Status = StatusIgnored
			continue
		}
		if e.Status != StatusFailed {
			continue
		}
		g := groups[e.Host]
		if len(g) == 0 || g[len(g)-1] != e.Seq {
			groups[e.Host] = append(g, e.Seq)
		}
	}
	for host, seqs := range groups {
		h, ok := recap[host]
		if !ok || len(seqs) <= h.Failed {
			continue
		}
		status := recovered(r.Input, h)
		if status == StatusFailed {
			continue
		}
		earlier := map[int]bool{}
		for _, s := range seqs[:len(seqs)-h.Failed] {
			earlier[s] = true
		}
		for i := range events {
			if events[i].Host == host && events[i].Status == StatusFailed && earlier[events[i].Seq] {
				events[i].Status = status
			}
		}
	}
	return events
}

func recovered(input string, h HostRecap) Status {
	switch {
	case input == "text" && h.Rescued > 0:
		return StatusRescued
	case input == "text":
		return StatusFailed
	case h.Rescued > 0 && h.Ignored > 0:
		return StatusRecovered
	case h.Rescued > 0:
		return StatusRescued
	case h.Ignored > 0:
		return StatusIgnored
	}
	return StatusFailed
}

func Build(r Run, o Options) Report {
	rep := Report{
		Tool: "ansible-playbook", ToolVersion: r.Version, Input: r.Input, Plays: r.Plays, Recap: r.HasRecap,
		Hosts: []HostRecap{}, Tasks: []Task{}, Errors: append([]string{}, r.Errors...), Warnings: append([]string{}, r.Warnings...),
	}
	others := 0
	for _, h := range r.Recap {
		if h.Failed > 0 {
			rep.Failed++
		}
		if h.Unreachable > 0 {
			rep.Unreachable++
		}
		if h.Failed+h.Unreachable > 0 {
			rep.Hosts = append(rep.Hosts, h)
			continue
		}
		if o.MaxHosts > 0 && others >= o.MaxHosts {
			rep.HostsOmitted++
			continue
		}
		others++
		rep.Hosts = append(rep.Hosts, h)
	}
	events := classify(r)
	failedHosts, unreachableHosts := map[string]bool{}, map[string]bool{}
	seen := map[Status]map[key]bool{}
	count := func(e Event) int {
		if seen[e.Status] == nil {
			seen[e.Status] = map[key]bool{}
		}
		k := key{e.Host, e.Seq}
		if seen[e.Status][k] {
			return 0
		}
		seen[e.Status][k] = true
		return 1
	}
	var problems, rest []Task
	for _, e := range events {
		t := Task{Status: e.Status, Host: e.Host, Play: e.Play, Task: e.Task, Path: e.Path, Item: e.Item, Detail: e.Detail}
		switch e.Status {
		case StatusOK, StatusSkipped:
		case StatusChanged:
			if count(e) == 0 {
				continue
			}
			rep.Changed++
			t.Item = ""
			rest = append(rest, t)
		case StatusFailed:
			failedHosts[e.Host] = true
			problems = append(problems, t)
		case StatusUnreachable:
			unreachableHosts[e.Host] = true
			problems = append(problems, t)
		case StatusIgnored:
			rep.Ignored += count(e)
			rest = append(rest, t)
		case StatusRescued:
			rep.Rescued += count(e)
			rest = append(rest, t)
		case StatusRecovered:
			rep.Recovered += count(e)
			rest = append(rest, t)
		}
	}
	if !r.HasRecap {
		rep.Failed, rep.Unreachable = len(failedHosts), len(unreachableHosts)
	}
	listed := problems
	if o.All {
		listed = append(listed, rest...)
	}
	shown := listed
	if o.MaxTasks > 0 && len(listed) > o.MaxTasks {
		shown, rep.TasksOmitted = listed[:o.MaxTasks], len(listed)-o.MaxTasks
	}
	rep.Tasks = append(rep.Tasks, shown...)
	return rep
}
