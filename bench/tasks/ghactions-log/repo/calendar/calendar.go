package calendar

import "time"

func NextBusinessDay(d time.Time) time.Time {
	switch d.Weekday() {
	case time.Saturday:
		return d.AddDate(0, 0, 1)
	case time.Sunday:
		return d.AddDate(0, 0, 1)
	}
	return d
}

func DueDate(issued time.Time, termDays int) time.Time {
	return NextBusinessDay(issued.AddDate(0, 0, termDays))
}
