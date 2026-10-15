package core

import "testing"

func TestNegatedFlagWords(t *testing.T) {
	t.Parallel()
	tests := map[string][2]bool{
		"error: layer missing":                            {true, false},
		"1 warning, 0 errors":                             {false, true},
		"0 warnings suppressed":                           {false, false},
		"[001/400] ok (3 symbols, 0 warnings suppressed)": {false, false},
		"no errors found":                                 {false, false},
		"build finished without warnings":                 {false, false},
		"errors: 0, warnings: 2":                          {false, true},
		"warnings=0":                                      {false, false},
		"non-fatal":                                       {false, false},
		"Fatal error":                                     {true, false},
		"12 failed, 3 passed":                             {true, false},
		"zero failures":                                   {false, false},
		"ValueError: invalid literal":                     {true, false},
		"E   AssertionError":                              {true, false},
		"Caused by: java.lang.IllegalStateException: x":   {true, false},
		"Uncaught TypeError: x is not a function":         {true, false},
		"registered onError handler":                      {false, false},
		"calling handleError(ctx)":                        {false, false},
		"MirrorError retries 3":                           {false, false},
	}
	for line, want := range tests {
		if got := [2]bool{ErrorLine(line), WarningLine(line)}; got != want {
			t.Errorf("%q: error, warning = %v, want %v", line, got, want)
		}
	}
}
