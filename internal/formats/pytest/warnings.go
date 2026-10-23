package pytest

import (
	"strconv"
	"strings"
)

type warningParser struct {
	locations int
	message   string
	closed    bool
	open      bool
	order     []string
	byMsg     map[string]int
}

func (w *warningParser) flush() {
	if w.open && (w.message != "" || w.locations > 0) {
		if w.byMsg == nil {
			w.byMsg = map[string]int{}
		}
		if _, seen := w.byMsg[w.message]; !seen {
			w.order = append(w.order, w.message)
		}
		w.byMsg[w.message] += w.locations
	}
	w.locations, w.message, w.closed, w.open = 0, "", false, false
}

func (w *warningParser) line(l string) {
	t := strings.TrimSpace(l)
	switch {
	case strings.HasPrefix(t, "-- Docs: "):
		w.flush()
	case t == "":
		w.closed = w.message != ""
	case !strings.HasPrefix(l, " ") && !isWarningLocation(t):
		if w.message != "" && w.locations == 0 && !w.closed {
			w.message += " " + t
			return
		}
		w.flush()
		w.open, w.message = true, t
	case !strings.HasPrefix(l, " "):
		if w.message != "" {
			w.flush()
		}
		w.open = true
		if m := locationCount.FindStringSubmatch(t); m != nil {
			n, _ := strconv.Atoi(m[1])
			w.locations += n
		} else {
			w.locations++
		}
	case strings.HasPrefix(l, "    "):
		w.closed = true
	case w.message == "":
		w.open, w.message = true, t
	case !w.closed:
		w.message += " " + t
	}
}

func isWarningLocation(t string) bool {
	return !strings.Contains(t, " ") || locationCount.MatchString(t)
}

func (w *warningParser) result() []Warning {
	w.flush()
	out := make([]Warning, 0, len(w.order))
	for _, msg := range w.order {
		out = append(out, Warning{Message: msg, Count: w.byMsg[msg]})
	}
	return out
}
