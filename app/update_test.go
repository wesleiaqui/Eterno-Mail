package app

import "testing"

func TestCompareReleaseVersions(t *testing.T) {
	tests := []struct {
		name    string
		left    string
		right   string
		want    int
		wantErr bool
	}{
		{name: "newer patch", left: "0.3.4", right: "0.3.3", want: 1},
		{name: "older patch", left: "0.3.2", right: "0.3.3", want: -1},
		{name: "same", left: "0.3.3", right: "0.3.3", want: 0},
		{name: "v prefix", left: "v1.0.0", right: "0.9.9", want: 1},
		{name: "major", left: "2.0.0", right: "1.99.99", want: 1},
		{name: "minor", left: "1.10.0", right: "1.9.9", want: 1},
		{name: "short version", left: "1.2", right: "1.2.0", want: 0},
		{name: "release metadata ignored", left: "1.2.3+linux", right: "1.2.3", want: 0},
		{name: "prerelease numeric core", left: "1.2.4-beta.1", right: "1.2.3", want: 1},
		{name: "invalid", left: "latest", right: "1.0.0", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareReleaseVersions(tt.left, tt.right)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("compareReleaseVersions(%q, %q) = %d; want %d", tt.left, tt.right, got, tt.want)
			}
		})
	}
}

func TestParseReleaseVersionRejectsTooManyComponents(t *testing.T) {
	if _, err := parseReleaseVersion("1.2.3.4"); err == nil {
		t.Fatal("expected too many version components to fail")
	}
}
