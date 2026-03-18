package packs

import (
	"fmt"
	"strings"
	"sync"
)

type packEntry struct {
	pack   *Pack
	source string
}

// PackRegistry holds all registered packs and provides lookup/expansion.
type PackRegistry struct {
	mu         sync.RWMutex
	entries    []*packEntry
	index      map[string]int      // id -> entries index
	categories map[string][]string // category -> []pack_id
}

var (
	registryOnce sync.Once
	defaultReg   *PackRegistry
)

// RegisterPack registers an already constructed pack while preserving lazy regex
// compilation inside the pack itself.
func (r *PackRegistry) RegisterPack(pack *Pack, source string) error {
	if pack == nil {
		return fmt.Errorf("register pack: nil pack")
	}
	if pack.ID == "" {
		return fmt.Errorf("register pack: empty id")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if idx, exists := r.index[pack.ID]; exists {
		existingSource := r.entries[idx].source
		if existingSource == "" {
			existingSource = "registry"
		}
		if source == "" {
			source = "registry"
		}
		return fmt.Errorf("duplicate pack id %q: keeping existing source %q, rejecting %q", pack.ID, existingSource, source)
	}

	r.entries = append(r.entries, &packEntry{
		pack:   pack,
		source: source,
	})
	idx := len(r.entries) - 1
	r.index[pack.ID] = idx
	cat := CategoryOf(pack.ID)
	r.categories[cat] = append(r.categories[cat], pack.ID)
	return nil
}

// NewEmptyRegistry builds an empty PackRegistry.
func NewEmptyRegistry() *PackRegistry {
	return &PackRegistry{
		index:      make(map[string]int),
		categories: make(map[string][]string),
	}
}

// DefaultRegistry returns the singleton registry.
// Note: It must be initialized (e.g., via allpacks.RegisterAll) before use if you want built-in packs.
func DefaultRegistry() *PackRegistry {
	registryOnce.Do(func() {
		defaultReg = NewEmptyRegistry()
	})
	return defaultReg
}

// Get returns a pack by ID. Returns nil if not found.
func (r *PackRegistry) Get(id string) *Pack {
	r.mu.RLock()
	defer r.mu.RUnlock()

	idx, ok := r.index[id]
	if !ok {
		return nil
	}
	return r.entries[idx].pack
}

// ExpandEnabled expands category names (e.g., "database") into all sub-pack IDs
// (e.g., "database.postgresql", "database.mysql", ...). Non-category IDs are
// passed through unchanged. Duplicates are removed.
func (r *PackRegistry) ExpandEnabled(ids []string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool, len(ids)*2)
	var result []string
	for _, id := range ids {
		if subs, ok := r.categories[id]; ok {
			for _, sub := range subs {
				if !seen[sub] {
					seen[sub] = true
					result = append(result, sub)
				}
			}
		} else if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// AllIDs returns all registered pack IDs.
func (r *PackRegistry) AllIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, len(r.entries))
	for i, e := range r.entries {
		ids[i] = e.pack.ID
	}
	return ids
}

// Count returns the total number of registered packs.
func (r *PackRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// ResolveEnabledSet expands enabled categories, removes disabled entries,
// and returns a deduplicated ordered list of active pack IDs plus a set for O(1) lookup.
func (r *PackRegistry) ResolveEnabledSet(enabled, disabled []string) (ids []string, set map[string]bool) {
	expandedIDs := r.ExpandEnabled(enabled)
	disabledSet := make(map[string]bool)
	for _, id := range disabled {
		for _, eid := range r.ExpandEnabled([]string{id}) {
			disabledSet[eid] = true
		}
	}
	set = make(map[string]bool, len(expandedIDs))
	for _, id := range expandedIDs {
		if !disabledSet[id] {
			ids = append(ids, id)
			set[id] = true
		}
	}
	return ids, set
}

// CategoryOf extracts the category from a pack ID (e.g., "database.postgresql" -> "database").
func CategoryOf(id string) string {
	if idx := strings.IndexByte(id, '.'); idx >= 0 {
		return id[:idx]
	}
	return id
}
