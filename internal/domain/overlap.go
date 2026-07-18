package domain

import (
	"path"
	"strings"
)

// OverlapSeverity ranks how urgent a path collision between two participants
// is. Values are ordered from lowest to highest severity but callers should
// compare by named constant rather than integer.
type OverlapSeverity string

const (
	SeverityInfo     OverlapSeverity = "info"
	SeverityWarning  OverlapSeverity = "warning"
	SeverityHigh     OverlapSeverity = "high"
	SeverityCritical OverlapSeverity = "critical"
)

// PathOverlap describes a single collision between one path declared by
// participant A and one declared by participant B. PathA and PathB are the
// normalized (path.Clean'd) forms of the inputs.
type PathOverlap struct {
	PathA    string          `json:"pathA"`
	PathB    string          `json:"pathB"`
	Severity OverlapSeverity `json:"severity"`
}

// ClassifyOverlap returns one PathOverlap for every collision between a and
// b. Inputs are treated as repo-relative POSIX paths. Paths that fail to
// normalize (escape via "..") or resolve to "." are silently dropped so a
// malformed intent cannot manufacture bogus overlaps.
func ClassifyOverlap(a, b []string) []PathOverlap {
	an := normalizePaths(a)
	bn := normalizePaths(b)
	if len(an) == 0 || len(bn) == 0 {
		return nil
	}
	var out []PathOverlap
	for _, pa := range an {
		for _, pb := range bn {
			sev, ok := classifyPair(pa, pb)
			if !ok {
				continue
			}
			out = append(out, PathOverlap{PathA: pa, PathB: pb, Severity: sev})
		}
	}
	return out
}

func normalizePaths(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		p = path.Clean(p)
		if p == "." || p == ".." || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func classifyPair(a, b string) (OverlapSeverity, bool) {
	if a == b {
		return SeverityHigh, true
	}
	if isWithin(a, b) || isWithin(b, a) {
		return SeverityWarning, true
	}
	return "", false
}

// isWithin reports whether child sits strictly inside parent when both are
// interpreted as slash-separated path components. Equal paths are handled by
// the caller and do not count as "within".
func isWithin(parent, child string) bool {
	if parent == child {
		return false
	}
	prefix := parent + "/"
	return strings.HasPrefix(child, prefix)
}
