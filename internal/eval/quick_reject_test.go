package eval

import (
	"testing"
)

func TestQuickReject_NoMatch(t *testing.T) {
	idx := NewEnabledKeywordIndex([]PackKeywords{
		{PackIndex: 0, Keywords: []string{"git"}},
		{PackIndex: 1, Keywords: []string{"rm", "/rm"}},
	})
	rejected, mask := idx.QuickReject("echo hello world")
	if !rejected {
		t.Errorf("expected rejected=true for 'echo hello world', mask=%v", mask)
	}
}

func TestQuickReject_GitMatch(t *testing.T) {
	idx := NewEnabledKeywordIndex([]PackKeywords{
		{PackIndex: 0, Keywords: []string{"git"}},
		{PackIndex: 1, Keywords: []string{"rm", "/rm"}},
	})
	rejected, mask := idx.QuickReject("git status")
	if rejected {
		t.Fatal("expected rejected=false for 'git status'")
	}
	if !isBitSet(mask, 0) {
		t.Error("expected pack 0 (git) bit set")
	}
	if isBitSet(mask, 1) {
		t.Error("expected pack 1 (rm) bit NOT set")
	}
}

func TestQuickReject_MultipleMatches(t *testing.T) {
	idx := NewEnabledKeywordIndex([]PackKeywords{
		{PackIndex: 0, Keywords: []string{"git"}},
		{PackIndex: 1, Keywords: []string{"rm", "/rm"}},
	})
	rejected, mask := idx.QuickReject("git rm file")
	if rejected {
		t.Fatal("expected rejected=false")
	}
	if !isBitSet(mask, 0) {
		t.Error("expected pack 0 (git) bit set")
	}
	if !isBitSet(mask, 1) {
		t.Error("expected pack 1 (rm) bit set")
	}
}

func TestQuickReject_EmptyIndex(t *testing.T) {
	idx := NewEnabledKeywordIndex(nil)
	rejected, _ := idx.QuickReject("anything")
	if !rejected {
		t.Error("expected rejected=true for empty index")
	}
}

func TestQuickReject_CaseInsensitiveKeyword(t *testing.T) {
	idx := NewEnabledKeywordIndex([]PackKeywords{
		{PackIndex: 0, Keywords: []string{"reg"}},
		{PackIndex: 1, Keywords: []string{"rm"}},
	})

	// Uppercase command should match lowercase keyword "reg"
	rejected, mask := idx.QuickReject("REG DELETE HKCU\\Software\\Test")
	if rejected {
		t.Fatal("expected rejected=false for 'REG DELETE' with keyword 'reg'")
	}
	if !isBitSet(mask, 0) {
		t.Error("expected pack 0 (reg) bit set")
	}
	if isBitSet(mask, 1) {
		t.Error("expected pack 1 (rm) bit NOT set")
	}

	// Mixed case should also match
	rejected2, mask2 := idx.QuickReject("Reg Delete HKCU\\Software\\Test")
	if rejected2 {
		t.Fatal("expected rejected=false for 'Reg Delete' with keyword 'reg'")
	}
	if !isBitSet(mask2, 0) {
		t.Error("expected pack 0 (reg) bit set for mixed case")
	}
}

func TestQuickReject_MixedCaseKeyword(t *testing.T) {
	// Keywords stored with mixed case (e.g., from windows pack) should still
	// match commands that use the original casing.
	idx := NewEnabledKeywordIndex([]PackKeywords{
		{PackIndex: 0, Keywords: []string{"Remove-Item"}},
		{PackIndex: 1, Keywords: []string{"Stop-Service"}},
	})

	// Original case command should match lowered keyword via case-insensitive AC.
	rejected, mask := idx.QuickReject("Remove-Item -Recurse -Force C:/Windows")
	if rejected {
		t.Fatal("expected rejected=false for 'Remove-Item' with keyword 'Remove-Item'")
	}
	if !isBitSet(mask, 0) {
		t.Error("expected pack 0 (Remove-Item) bit set")
	}
	if isBitSet(mask, 1) {
		t.Error("expected pack 1 (Stop-Service) bit NOT set")
	}

	// Lowercase command should also match.
	rejected2, mask2 := idx.QuickReject("remove-item -recurse -force c:/windows")
	if rejected2 {
		t.Fatal("expected rejected=false for lowercase 'remove-item'")
	}
	if !isBitSet(mask2, 0) {
		t.Error("expected pack 0 bit set for lowercase command")
	}

	// ALL-CAPS command should match too.
	rejected3, mask3 := idx.QuickReject("REMOVE-ITEM -RECURSE -FORCE C:/WINDOWS")
	if rejected3 {
		t.Fatal("expected rejected=false for uppercase 'REMOVE-ITEM'")
	}
	if !isBitSet(mask3, 0) {
		t.Error("expected pack 0 bit set for uppercase command")
	}
}
