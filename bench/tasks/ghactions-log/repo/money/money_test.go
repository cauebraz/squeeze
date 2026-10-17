package money

import "testing"

func TestString(t *testing.T) {
	cases := map[Cents]string{0: "$0.00", 5: "$0.05", 1999: "$19.99", -250: "-$2.50", 100000: "$1000.00"}
	for in, want := range cases {
		if got := in.String(); got != want {
			t.Errorf("Cents(%d).String() = %q, want %q", int64(in), got, want)
		}
	}
}

func TestParse(t *testing.T) {
	cases := map[string]Cents{"$0.00": 0, "19.99": 1999, "-$2.50": -250, "7": 700, " $12.30 ": 1230}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %d, %v, want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"1.5", "abc", "$1.2x"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) returned no error", bad)
		}
	}
}
