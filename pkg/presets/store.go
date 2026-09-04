package presets

import (
	"fmt"
	"strings"
	"sync"
)

// Store holds user-created cultural presets alongside the four built-in ones. It exists because
// "what a traffic spike looks like" is not universal: an operator running Oshimai outside
// Indonesia has their own equivalents of Harbolnas/Gajian/Mudik (Black Friday, Singles' Day
// 11.11, Boxing Day, a local payday cadence, ...) and shouldn't need a code change and a redeploy
// to model them. Built-in presets are always present and always read-only through a Store —
// Create/Update/Delete only ever touch custom entries, so the defaults can never be silently
// edited out from under a fresh install.
type Store struct {
	mu     sync.RWMutex
	custom map[string]*CulturalPreset
	order  []string // Preserves creation order for List, since map iteration order is not stable.
}

// NewStore creates an empty custom-preset store. The four built-ins are available regardless,
// through List/Find, even before anything is added.
func NewStore() *Store {
	return &Store{custom: make(map[string]*CulturalPreset)}
}

// List returns every preset: the built-ins first (fixed order), then custom presets in the order
// they were created.
func (s *Store) List() []CulturalPreset {
	out := List()
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, id := range s.order {
		out = append(out, *s.custom[id])
	}
	return out
}

// Find looks up any preset, built-in or custom, by ID.
func (s *Store) Find(id string) (CulturalPreset, bool) {
	if p, ok := Find(id); ok {
		return p, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.custom[id]; ok {
		return *p, true
	}
	return CulturalPreset{}, false
}

// Create validates and adds a new custom preset. If p.ID is empty, or collides with an existing
// built-in or custom preset, an ID is derived from Name (and de-duplicated) instead — the caller
// never has to pre-check availability.
func (s *Store) Create(p CulturalPreset) (CulturalPreset, error) {
	if err := validatePreset(p); err != nil {
		return CulturalPreset{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := slugify(p.ID)
	if id == "" || s.idTakenLocked(id) {
		id = slugify(p.Name)
	}
	if id == "" {
		id = "custom_preset"
	}
	base := id
	for n := 1; s.idTakenLocked(id); n++ {
		id = fmt.Sprintf("%s_%d", base, n)
	}

	p.ID = id
	p.Custom = true
	s.custom[id] = &p
	s.order = append(s.order, id)
	return p, nil
}

// Update replaces a custom preset's editable fields in place. Built-in presets can't be edited
// through this method — that's the entire point of keeping them separate from custom ones.
func (s *Store) Update(id string, p CulturalPreset) (CulturalPreset, error) {
	if _, ok := Find(id); ok {
		return CulturalPreset{}, fmt.Errorf("%q is a built-in preset and cannot be modified", id)
	}
	if err := validatePreset(p); err != nil {
		return CulturalPreset{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.custom[id]; !ok {
		return CulturalPreset{}, fmt.Errorf("custom preset %q not found", id)
	}
	p.ID = id
	p.Custom = true
	s.custom[id] = &p
	return p, nil
}

// Delete removes a custom preset. Built-in presets can't be deleted.
func (s *Store) Delete(id string) error {
	if _, ok := Find(id); ok {
		return fmt.Errorf("%q is a built-in preset and cannot be deleted", id)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.custom[id]; !ok {
		return fmt.Errorf("custom preset %q not found", id)
	}
	delete(s.custom, id)
	for i, oid := range s.order {
		if oid == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	return nil
}

// idTakenLocked reports whether id is already used by a built-in or a custom preset. Caller must
// hold s.mu (read or write).
func (s *Store) idTakenLocked(id string) bool {
	if _, ok := Find(id); ok {
		return true
	}
	_, ok := s.custom[id]
	return ok
}

func validatePreset(p CulturalPreset) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if p.PeakMultiplier < 1 {
		return fmt.Errorf("peak_multiplier must be at least 1")
	}
	if p.SuggestedTotalS < 1 {
		return fmt.Errorf("suggested_total_duration_sec must be positive")
	}
	if len(p.Shape) > 0 {
		sum := 0
		for _, st := range p.Shape {
			if st.DurationPercent < 0 || st.VUMultiplier < 0 {
				return fmt.Errorf("shape stages must have non-negative duration_percent and vu_multiplier")
			}
			sum += st.DurationPercent
		}
		if sum != 100 {
			return fmt.Errorf("shape stage duration_percent values must sum to 100, got %d", sum)
		}
	}
	return nil
}

// slugify turns arbitrary user input into a lowercase, underscore-separated identifier safe to
// use as a preset ID and as a URL path segment.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.TrimRight(b.String(), "_")
}
