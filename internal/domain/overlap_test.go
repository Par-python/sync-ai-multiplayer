package domain

import "testing"

func TestClassifyOverlap(t *testing.T) {
	got := ClassifyOverlap([]string{"packages/auth"}, []string{"packages/auth/client.ts"})
	if len(got) != 1 || got[0].Severity != SeverityWarning {
		t.Fatalf("got %#v", got)
	}
}

func TestClassifyOverlapSameFileIsHigh(t *testing.T) {
	got := ClassifyOverlap([]string{"go.mod"}, []string{"go.mod"})
	if len(got) != 1 || got[0].Severity != SeverityHigh {
		t.Fatalf("got %#v", got)
	}
}

func TestClassifyOverlapDisjointReturnsEmpty(t *testing.T) {
	got := ClassifyOverlap([]string{"packages/auth"}, []string{"packages/billing"})
	if len(got) != 0 {
		t.Fatalf("expected no overlap, got %#v", got)
	}
}

func TestClassifyOverlapRejectsEscapingPaths(t *testing.T) {
	got := ClassifyOverlap([]string{"../etc"}, []string{"packages/auth"})
	if len(got) != 0 {
		t.Fatalf("expected escaping paths to be dropped, got %#v", got)
	}
}
