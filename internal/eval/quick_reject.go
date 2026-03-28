// Package eval provides the command evaluation pipeline.
package eval

import (
	"strings"

	ahocorasick "github.com/petar-dambovaliev/aho-corasick"
)

// PackKeywords carries the keyword list for one pack.
type PackKeywords struct {
	PackIndex int
	Keywords  []string
}

// EnabledKeywordIndex is built once at startup from the enabled packs' keyword
// lists and reused for every command evaluation.
type EnabledKeywordIndex struct {
	ac             *ahocorasick.AhoCorasick
	keywords       []string   // keyword for each pattern index (used for boundary check)
	patternMasks   []packMask // pattern index → pack bitmask
	fullMask       packMask   // OR of all pack masks
	noKeywordsMask packMask   // bits for packs with zero keywords (always candidates)
}

type packMask struct {
	words []uint64
}

func newPackMask(size int) packMask {
	if size <= 0 {
		return packMask{}
	}
	return packMask{words: make([]uint64, (size+63)/64)}
}

func (m packMask) set(i int) {
	if i < 0 {
		return
	}
	word := i / 64
	if word >= len(m.words) {
		return
	}
	m.words[word] |= uint64(1) << (i % 64)
}

func (m packMask) isSet(i int) bool {
	if i < 0 {
		return false
	}
	word := i / 64
	if word >= len(m.words) {
		return false
	}
	return m.words[word]&(uint64(1)<<(i%64)) != 0
}

func (m packMask) or(other packMask) {
	for i := range m.words {
		m.words[i] |= other.words[i]
	}
}

func (m packMask) isZero() bool {
	for _, word := range m.words {
		if word != 0 {
			return false
		}
	}
	return true
}

func (m packMask) equals(other packMask) bool {
	if len(m.words) != len(other.words) {
		return false
	}
	for i := range m.words {
		if m.words[i] != other.words[i] {
			return false
		}
	}
	return true
}

// NewEnabledKeywordIndex builds the index from a slice of (packIndex, keywords) pairs.
func NewEnabledKeywordIndex(packs []PackKeywords) *EnabledKeywordIndex {
	maskSize := len(packs)
	for _, pk := range packs {
		if pk.PackIndex+1 > maskSize {
			maskSize = pk.PackIndex + 1
		}
	}
	fullMask := newPackMask(maskSize)
	noKeywordsMask := newPackMask(maskSize)

	// Map keyword to merged bitmask. This ensures that if multiple packs
	// share a keyword (e.g. "rm"), all of them are considered as candidates.
	mergedMasks := make(map[string]packMask)

	for _, pk := range packs {
		fullMask.set(pk.PackIndex)

		if len(pk.Keywords) == 0 {
			// Packs with no keywords can't be quick-rejected — always candidates.
			noKeywordsMask.set(pk.PackIndex)
		}

		for _, kw := range pk.Keywords {
			kw = strings.ToLower(kw)
			m := mergedMasks[kw]
			if len(m.words) == 0 {
				m = newPackMask(maskSize)
			}
			m.set(pk.PackIndex)
			mergedMasks[kw] = m
		}
	}

	patterns := make([]string, 0, len(mergedMasks))
	keywords := make([]string, 0, len(mergedMasks))
	masks := make([]packMask, 0, len(mergedMasks))

	// We need a stable order for patterns to match them back to masks.
	// Map iteration is random, but that's fine as long as we build
	// the slices consistently.
	for kw, m := range mergedMasks {
		patterns = append(patterns, kw)
		keywords = append(keywords, kw)
		masks = append(masks, m)
	}

	idx := &EnabledKeywordIndex{
		keywords:       keywords,
		patternMasks:   masks,
		fullMask:       fullMask,
		noKeywordsMask: noKeywordsMask,
	}

	if len(patterns) > 0 {
		builder := ahocorasick.NewAhoCorasickBuilder(ahocorasick.Opts{
			DFA:                  true,
			AsciiCaseInsensitive: true,
		})
		ac := builder.Build(patterns)
		idx.ac = &ac
	}

	return idx
}

// QuickReject runs the keyword scan on cmd.
// Returns rejected=true if no keywords matched (caller should allow).
// Matches are word-boundary-aware: a keyword like "git" won't match inside
// "digit" because the preceding character is a word character.
func (idx *EnabledKeywordIndex) QuickReject(cmd string) (rejected bool, candidateMask packMask) {
	if idx.ac == nil {
		// If there are no keywords indexed, only reject if there are also no
		// packs that have zero keywords (noKeywordsMask).
		if idx.noKeywordsMask.isZero() {
			return true, packMask{}
		}
		return false, idx.noKeywordsMask
	}

	// Lowercase the command for word-boundary checks. Keywords are lowercased
	// during index construction and the AC automaton is built with
	// AsciiCaseInsensitive, so matching is case-insensitive. The keyword index
	// is only used for candidate filtering — actual regex matching happens on
	// the original command — so this is safe and correct.
	lowerCmd := strings.ToLower(cmd)
	iter := idx.ac.IterOverlapping(lowerCmd)
	mask := newPackMask(len(idx.fullMask.words) * 64)

	for next := iter.Next(); next != nil; next = iter.Next() {
		patIdx := next.Pattern()
		if patIdx >= len(idx.patternMasks) {
			continue
		}

		// Word-boundary check: keywords that start/end with word characters
		// must be at word boundaries in the command string. This prevents
		// "digit" from matching the "git" keyword. Keywords containing
		// whitespace or special characters (like "/dev/") skip this check
		// because they already have implicit boundaries.
		kw := idx.keywords[patIdx]
		if !isWordBoundaryMatch(lowerCmd, next.Start(), next.End(), kw) {
			continue
		}

		mask.or(idx.patternMasks[patIdx])

		// Early exit if all packs are candidates.
		if mask.equals(idx.fullMask) {
			break
		}
	}

	// Always include packs with no keywords — they can't be quick-rejected.
	mask.or(idx.noKeywordsMask)

	if mask.isZero() {
		return true, mask
	}
	return false, mask
}

// isWordByte returns true if b is a word character (letter, digit, underscore).
func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// isWordBoundaryMatch checks whether the keyword match at [start, end) in cmd
// is at a word boundary. If the keyword's first character is a word character,
// the character before start must not be a word character (or start must be 0).
// Similarly for the last character and end position.
func isWordBoundaryMatch(cmd string, start, end int, kw string) bool {
	if len(kw) == 0 {
		return false
	}

	// If the keyword contains whitespace or non-word characters at boundaries,
	// the boundary is implicit — don't require word boundary.
	firstIsWord := isWordByte(kw[0])
	lastIsWord := isWordByte(kw[len(kw)-1])

	if firstIsWord && start > 0 && isWordByte(cmd[start-1]) {
		return false
	}
	if lastIsWord && end < len(cmd) && isWordByte(cmd[end]) {
		return false
	}
	return true
}
