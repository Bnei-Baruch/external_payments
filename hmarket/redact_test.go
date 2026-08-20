package hmarket

import "testing"

func TestMaskEmail(t *testing.T) {
	for in, want := range map[string]string{
		"sharetalona@gmail.com": "s***@gmail.com",
		"a@b.co":                "a***@b.co",
		"notanemail":            "***",
		"@nolocal.com":          "***",
		"":                      "",
	} {
		if got := maskEmail(in); got != want {
			t.Errorf("maskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaskPhone(t *testing.T) {
	for in, want := range map[string]string{
		"052-774-8025":  "***8025",
		"+972500000000": "***0000",
		"123":           "***",
		"abc":           "",
		"":              "",
	} {
		if got := maskPhone(in); got != want {
			t.Errorf("maskPhone(%q) = %q, want %q", in, got, want)
		}
	}
}
