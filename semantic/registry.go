// Package semantic — SkillRegistry interface, snapshot, and fingerprinting
// per SPEC v0.3 §12, §16.3.
package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

// SkillRegistry is the injectable interface for skill resolution (SPEC v0.3 §12).
// It replaces the hardcoded map as the production registry.
type SkillRegistry interface {
	Resolve(ctx context.Context, id, version string) (SkillSpec, error)
	SnapshotDigest(ctx context.Context) (string, error)
}

// StaticRegistry implements SkillRegistry with an in-memory map.
// Suitable for development and testing. Production should use an
// immutable back-end (SQLite, OCI registry, etc.).
type StaticRegistry struct {
	skills map[string]SkillSpec
}

// NewStaticRegistry creates a StaticRegistry preloaded from a map.
func NewStaticRegistry(skills map[string]SkillSpec) *StaticRegistry {
	sr := &StaticRegistry{skills: make(map[string]SkillSpec, len(skills))}
	for name, s := range skills {
		sr.skills[name] = s
	}
	return sr
}

// Resolve looks up a skill by ID and optional version.
func (r *StaticRegistry) Resolve(_ context.Context, id, version string) (SkillSpec, error) {
	spec, ok := r.skills[id]
	if !ok {
		return SkillSpec{}, fmt.Errorf("skill %q not found in registry", id)
	}
	if version != "" && spec.Version != version {
		return SkillSpec{}, fmt.Errorf("skill %q version %q not found (have %s)", id, version, spec.Version)
	}
	return spec, nil
}

// SnapshotDigest returns a cryptographic hash of the full registry contents
// at this point in time, ensuring reproducibility.
func (r *StaticRegistry) SnapshotDigest(_ context.Context) (string, error) {
	// Deterministic serialization: sort keys, canonical JSON.
	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	sort.Strings(names)

	type snapshotEntry struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Digest  string `json:"digest"`
		Runtime string `json:"runtime,omitempty"`
	}
	entries := make([]snapshotEntry, 0, len(names))
	for _, name := range names {
		s := r.skills[name]
		entries = append(entries, snapshotEntry{
			Name:    s.Name,
			Version: s.Version,
			Digest:  s.Digest,
			Runtime: s.Runtime,
		})
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("snapshot marshal: %w", err)
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h), nil
}

// Fingerprint computes the flow compilation fingerprint per SPEC v0.3 §16.3:
//
//	SHA-256(canonical IR + compiler version + registry snapshot + trust policy version)
func Fingerprint(irJSON []byte, compilerVersion, registrySnapshot, trustPolicyVersion string) string {
	h := sha256.New()
	h.Write(irJSON)
	h.Write([]byte(compilerVersion))
	h.Write([]byte(registrySnapshot))
	h.Write([]byte(trustPolicyVersion))
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

// ensure StaticRegistry satisfies SkillRegistry.
var _ SkillRegistry = (*StaticRegistry)(nil)
