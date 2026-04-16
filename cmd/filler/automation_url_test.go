package main

import "testing"

func TestNormalizeApplicationJobURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "relative linkedin path", in: "/jobs/view/123", want: "https://www.linkedin.com/jobs/view/123"},
		{name: "scheme-relative linkedin host", in: "//www.linkedin.com/jobs/view/456", want: "https://www.linkedin.com/jobs/view/456"},
		{name: "already absolute", in: "https://www.linkedin.com/jobs/view/789", want: "https://www.linkedin.com/jobs/view/789"},
		{name: "empty", in: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeApplicationJobURL(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeApplicationJobURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
