package quiz

import (
	"testing"
)

func TestParseForLocale(t *testing.T) {
	tr := `[{"q":"TR Soru","options":["A","B"],"correct":0}]`
	en := `[{"q":"EN Question","options":["A","B"],"correct":0}]`
	bg := `[{"q":"BG Въпрос","options":["A","B"],"correct":0}]`

	// Test with TR locale
	qs := ParseForLocale(tr, en, bg, "tr")
	if len(qs) != 1 || qs[0].Q != "TR Soru" {
		t.Errorf("TR locale returned wrong quiz: %v", qs)
	}

	// Test with EN locale
	qs = ParseForLocale(tr, en, bg, "en")
	if len(qs) != 1 || qs[0].Q != "EN Question" {
		t.Errorf("EN locale returned wrong quiz: %v", qs)
	}

	// Test with BG locale
	qs = ParseForLocale(tr, en, bg, "bg")
	if len(qs) != 1 || qs[0].Q != "BG Въпрос" {
		t.Errorf("BG locale returned wrong quiz: %v", qs)
	}

	// Test fallback when EN is empty
	qs = ParseForLocale(tr, "", "", "en")
	if len(qs) != 1 || qs[0].Q != "TR Soru" {
		t.Errorf("Fallback to TR failed: %v", qs)
	}

	// Test fallback when EN is empty and BG is empty for BG locale
	qs = ParseForLocale(tr, "", "", "bg")
	if len(qs) != 1 || qs[0].Q != "TR Soru" {
		t.Errorf("Fallback to TR for BG locale failed: %v", qs)
	}

	// Test unknown locale → should use TR
	qs = ParseForLocale(tr, en, bg, "fr")
	if len(qs) != 1 || qs[0].Q != "TR Soru" {
		t.Errorf("Unknown locale should fallback to TR: %v", qs)
	}

	// Test all locales return same count
	for _, loc := range []string{"tr", "en", "bg"} {
		qs = ParseForLocale(tr, en, bg, loc)
		if len(qs) != 1 {
			t.Errorf("Locale %s: expected 1 question, got %d", loc, len(qs))
		}
	}

	// Test 10 iterations (user requirement)
	for i := 0; i < 10; i++ {
		qs = ParseForLocale(tr, en, bg, "en")
		if len(qs) != 1 || qs[0].Q != "EN Question" {
			t.Errorf("Iteration %d: consistency failure", i)
		}
	}
}

func TestParse(t *testing.T) {
	// Normal case
	qs := Parse(`[{"q":"Test?","options":["A","B","C"],"correct":1}]`)
	if len(qs) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qs))
	}
	if qs[0].Q != "Test?" {
		t.Errorf("wrong question text: %s", qs[0].Q)
	}
	if len(qs[0].Options) != 3 {
		t.Errorf("wrong option count: %d", len(qs[0].Options))
	}

	// Empty
	if qs := Parse(""); len(qs) != 0 {
		t.Error("empty should return nil")
	}

	// Whitespace only
	if qs := Parse("   "); len(qs) != 0 {
		t.Error("whitespace should return nil")
	}

	// Invalid JSON
	if qs := Parse("invalid"); len(qs) != 0 {
		t.Error("invalid JSON should return nil")
	}

	// Empty question text filtered
	qs = Parse(`[{"q":"","options":["A","B"],"correct":0}]`)
	if len(qs) != 0 {
		t.Error("empty question text should be filtered")
	}

	// Too few options filtered
	qs = Parse(`[{"q":"Test?","options":["A"],"correct":0}]`)
	if len(qs) != 0 {
		t.Error("question with <2 options should be filtered")
	}

	// Out of range correct index filtered
	qs = Parse(`[{"q":"Test?","options":["A","B"],"correct":5}]`)
	if len(qs) != 0 {
		t.Error("out of range correct index should be filtered")
	}
}

func TestGrade(t *testing.T) {
	qs := []Question{
		{Q: "Q1", Options: []string{"A", "B", "C"}, Correct: 1},
		{Q: "Q2", Options: []string{"X", "Y"}, Correct: 0},
	}

	// All correct
	c, tot := Grade(qs, []int{1, 0})
	if c != 2 || tot != 2 {
		t.Errorf("all correct: got %d/%d, expected 2/2", c, tot)
	}

	// All wrong
	c, tot = Grade(qs, []int{0, 1})
	if c != 0 || tot != 2 {
		t.Errorf("all wrong: got %d/%d, expected 0/2", c, tot)
	}

	// Partial
	c, tot = Grade(qs, []int{1, 1})
	if c != 1 || tot != 2 {
		t.Errorf("partial: got %d/%d, expected 1/2", c, tot)
	}

	// Too few answers
	c, tot = Grade(qs, []int{1})
	if c != 1 || tot != 2 {
		t.Errorf("too few: got %d/%d, expected 1/2", c, tot)
	}

	// Empty
	c, tot = Grade([]Question{}, []int{})
	if c != 0 || tot != 0 {
		t.Error("empty should return 0/0")
	}
}
