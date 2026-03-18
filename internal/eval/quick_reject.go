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
	ac           *ahocorasick.AhoCorasick
	keywords     []string    // keyword for each pattern index (used for boundary check)
	patternMasks [][2]uint64 // pattern index → pack bitmask
	fullMask     [2]uint64   // OR of all pack masks
}

// maxPacks is the maximum number of packs supported by the [2]uint64 bitmask.
// To support more packs, widen the array (e.g. [4]uint64 for 256 packs) and
// update isBitSet in evaluator.go accordingly.
const maxPacks = 128

// NewEnabledKeywordIndex builds the index from a slice of (packIndex, keywords) pairs.
func NewEnabledKeywordIndex(packs []PackKeywords) *EnabledKeywordIndex {
	var fullMask [2]uint64

	// Map keyword to merged bitmask. This ensures that if multiple packs
	// share a keyword (e.g. "rm"), all of them are considered as candidates.
	mergedMasks := make(map[string][2]uint64)

	for _, pk := range packs {
		if pk.PackIndex >= maxPacks {
			continue // silently skip — fail-open; see maxPacks comment
		}
		word := pk.PackIndex / 64
		bit := uint64(1) << (pk.PackIndex % 64)
		fullMask[word] |= bit

		for _, kw := range pk.Keywords {
			kw = strings.ToLower(kw)
			m := mergedMasks[kw]
			m[word] |= bit
			mergedMasks[kw] = m
		}
	}

	var patterns []string
	var keywords []string
	var masks [][2]uint64

	// We need a stable order for patterns to match them back to masks.
	// Map iteration is random, but that's fine as long as we build
	// the slices consistently.
	for kw, m := range mergedMasks {
		patterns = append(patterns, kw)
		keywords = append(keywords, kw)
		masks = append(masks, m)
	}

	idx := &EnabledKeywordIndex{
		keywords:     keywords,
		patternMasks: masks,
		fullMask:     fullMask,
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
func (idx *EnabledKeywordIndex) QuickReject(cmd string) (rejected bool, candidateMask [2]uint64) {
	if idx.ac == nil {
		return true, [2]uint64{}
	}

	// Lowercase the command for word-boundary checks. Keywords are lowercased
	// during index construction and the AC automaton is built with
	// AsciiCaseInsensitive, so matching is case-insensitive. The keyword index
	// is only used for candidate filtering — actual regex matching happens on
	// the original command — so this is safe and correct.
	lowerCmd := strings.ToLower(cmd)
	iter := idx.ac.Iter(lowerCmd)
	var mask [2]uint64

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

		mask[0] |= idx.patternMasks[patIdx][0]
		mask[1] |= idx.patternMasks[patIdx][1]

		// Early exit if all packs are candidates.
		if mask == idx.fullMask {
			break
		}
	}

	if mask == [2]uint64{} {
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
