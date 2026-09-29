package policy

import (
	"fmt"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
)

// Policy defines threshold limits and architectural rules.
type Policy struct {
	MaxInitial          *int64   `json:"maxInitial,omitempty"`
	MaxTotal            *int64   `json:"maxTotal,omitempty"`
	MaxInitialDelta     *int64   `json:"maxInitialDelta,omitempty"`
	ForbiddenPkgs       []string `json:"forbiddenPkgs,omitempty"`
	DetectDuplicatePkgs bool     `json:"detectDuplicatePkgs"`
}

// Violation represents an individual rule breach.
type Violation struct {
	Severity string `json:"severity"` // "error" or "warning"
	Rule     string `json:"rule"`     // e.g. "MAX_INITIAL_SIZE", "FORBIDDEN_PACKAGE"
	Message  string `json:"message"`
	Actual   int64  `json:"actual,omitempty"`
	Limit    int64  `json:"limit,omitempty"`
}

// EvaluationResult aggregates all evaluation findings.
type EvaluationResult struct {
	Passed     bool        `json:"passed"`
	Violations []Violation `json:"violations"`
	Warnings   []Violation `json:"warnings"`
}

// Evaluate checks a bundle and optional diff against a configured policy.
func Evaluate(bundle *core.Bundle, d *diff.BundleDiff, p Policy) EvaluationResult {
	res := EvaluationResult{
		Passed:     true,
		Violations: make([]Violation, 0),
		Warnings:   make([]Violation, 0),
	}

	if bundle == nil {
		return res
	}

	// 1. MaxInitial check (per entrypoint and overall)
	if p.MaxInitial != nil {
		limit := *p.MaxInitial
		for name, ep := range bundle.Entrypoints {
			if ep.InitialBytes > limit {
				res.Passed = false
				res.Violations = append(res.Violations, Violation{
					Severity: "error",
					Rule:     "MAX_INITIAL_SIZE",
					Message:  fmt.Sprintf("Entrypoint %q initial JS size %d bytes exceeds budget limit %d bytes", name, ep.InitialBytes, limit),
					Actual:   ep.InitialBytes,
					Limit:    limit,
				})
			}
		}
	}

	// 2. MaxTotal check
	if p.MaxTotal != nil {
		limit := *p.MaxTotal
		var totalBytes int64
		for _, c := range bundle.Chunks {
			totalBytes += c.SizeBytes
		}
		if totalBytes > limit {
			res.Passed = false
			res.Violations = append(res.Violations, Violation{
				Severity: "error",
				Rule:     "MAX_TOTAL_SIZE",
				Message:  fmt.Sprintf("Total JS bundle size %d bytes exceeds budget limit %d bytes", totalBytes, limit),
				Actual:   totalBytes,
				Limit:    limit,
			})
		}
	}

	// 3. MaxInitialDelta regression check
	if p.MaxInitialDelta != nil && d == nil {
		res.Passed = false
		res.Violations = append(res.Violations, Violation{
			Severity: "error",
			Rule:     "BASELINE_REQUIRED",
			Message:  "Initial delta budget requires a baseline diff",
		})
	} else if p.MaxInitialDelta != nil {
		limit := *p.MaxInitialDelta
		for name, epDelta := range d.Entrypoints {
			if epDelta.InitialDelta > limit {
				res.Passed = false
				res.Violations = append(res.Violations, Violation{
					Severity: "error",
					Rule:     "MAX_INITIAL_DELTA",
					Message:  fmt.Sprintf("Entrypoint %q initial JS regression +%d bytes exceeds delta limit %d bytes", name, epDelta.InitialDelta, limit),
					Actual:   epDelta.InitialDelta,
					Limit:    limit,
				})
			}
		}
	}

	// 4. Forbidden packages check
	if len(p.ForbiddenPkgs) > 0 {
		forbiddenMap := make(map[string]bool)
		for _, pkg := range p.ForbiddenPkgs {
			forbiddenMap[strings.ToLower(strings.TrimSpace(pkg))] = true
		}

		seenForbidden := make(map[string]bool)
		for _, m := range bundle.Modules {
			if m.Package == "" {
				continue
			}
			lowerPkg := strings.ToLower(m.Package)
			if forbiddenMap[lowerPkg] && !seenForbidden[lowerPkg] {
				seenForbidden[lowerPkg] = true
				res.Passed = false
				res.Violations = append(res.Violations, Violation{
					Severity: "error",
					Rule:     "FORBIDDEN_PACKAGE",
					Message:  fmt.Sprintf("Forbidden package %q was detected in compiled bundle", m.Package),
				})
			}
		}
	}

	// 5. Duplicate package versions check
	if p.DetectDuplicatePkgs {
		pkgVersions := make(map[string]map[string]bool)
		for _, m := range bundle.Modules {
			if m.Package == "" || m.Version == "" {
				continue
			}
			if _, ok := pkgVersions[m.Package]; !ok {
				pkgVersions[m.Package] = make(map[string]bool)
			}
			pkgVersions[m.Package][m.Version] = true
		}

		for pkg, versions := range pkgVersions {
			if len(versions) > 1 {
				vList := make([]string, 0, len(versions))
				for v := range versions {
					vList = append(vList, v)
				}
				res.Passed = false
				res.Violations = append(res.Violations, Violation{
					Severity: "error",
					Rule:     "DUPLICATE_PACKAGE_VER",
					Message:  fmt.Sprintf("Multiple bundled versions of %q detected: %s", pkg, strings.Join(vList, ", ")),
				})
			}
		}
	}

	return res
}
