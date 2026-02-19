package version

import "testing"

func TestDetailedIncludesAllBuildFields(t *testing.T) {
	oldVersion, oldCommit, oldDate := Version, Commit, Date
	defer func() {
		Version, Commit, Date = oldVersion, oldCommit, oldDate
	}()

	Version = "1.2.3"
	Commit = "abc1234"
	Date = "2026-02-19T00:00:00Z"

	want := "version: 1.2.3\ncommit: abc1234\nbuild date: 2026-02-19T00:00:00Z"
	if got := Detailed(); got != want {
		t.Fatalf("Detailed() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}
