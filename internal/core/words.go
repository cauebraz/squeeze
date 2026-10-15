package core

import "regexp"

var (
	errorWords   = regexp.MustCompile(`(?i)\b(errors?|fail(s|ed|ures?)?|fatal|panic(s|ked)?|exceptions?|tracebacks?)\b`)
	warningWords = regexp.MustCompile(`(?i)\b(warn(s|ed|ings?)?)\b`)
	negated      = regexp.MustCompile(`(?i)(^|[^\w])(0|zero|no|without|non)[\s-]*$`)
	zeroAfter    = regexp.MustCompile(`^\s*[:=]\s*0\b`)
	errorName    = regexp.MustCompile(`\b[A-Z]\w*(Error|Exception)(:|$)`)
)

func flagged(line string, words *regexp.Regexp) bool {
	for _, m := range words.FindAllStringIndex(line, -1) {
		if negated.MatchString(line[:m[0]]) || zeroAfter.MatchString(line[m[1]:]) {
			continue
		}
		return true
	}
	return false
}

func ErrorLine(line string) bool { return flagged(line, errorWords) || errorName.MatchString(line) }

func WarningLine(line string) bool { return flagged(line, warningWords) }
