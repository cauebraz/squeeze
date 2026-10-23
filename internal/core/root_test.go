package core

import "testing"

func TestTrimRoot(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, root, want string }{
		{"at f (/home/dev/shop/src/a.js:1:2)", "/home/dev/shop", "at f (src/a.js:1:2)"},
		{"see file:///home/dev/shop/src/a.js and /home/dev/shop/b.py", "/home/dev/shop", "see src/a.js and b.py"},
		{"/home/dev/shopping/a.js", "/home/dev/shop", "/home/dev/shopping/a.js"},
		{"/home/dev/shop/a.js", "", "/home/dev/shop/a.js"},
	}
	for _, tt := range tests {
		if got := TrimRoot(tt.in, tt.root); got != tt.want {
			t.Errorf("TrimRoot(%q, %q) = %q, want %q", tt.in, tt.root, got, tt.want)
		}
	}
}
