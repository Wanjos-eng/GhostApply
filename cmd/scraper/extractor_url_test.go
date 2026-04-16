package main

import "testing"

func TestNormalizeLinkedInURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "relative path", in: "/jobs/view/111", want: "https://www.linkedin.com/jobs/view/111"},
		{name: "scheme-relative", in: "//www.linkedin.com/jobs/view/222", want: "https://www.linkedin.com/jobs/view/222"},
		{name: "absolute", in: "https://www.linkedin.com/jobs/view/333", want: "https://www.linkedin.com/jobs/view/333"},
		{name: "empty", in: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeLinkedInURL(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeLinkedInURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
