package models

import (
	"testing"
)

func TestListSheets(t *testing.T) {
	names := ListSheets()
	want := []string{"ams151", "ams161", "ams210", "ams261", "cse113", "cse213", "phy131", "psy103"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v, want %v", names, want)
		}
	}
}

func TestLoadAllCounts(t *testing.T) {
	sheets, err := LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(sheets) != 8 {
		t.Fatalf("expected 8 sheets, got %d", len(sheets))
	}
	want := map[string][3]string{
		"ams151": {"AMS 151", "Applied Calculus I", ""},
		"ams161": {"AMS 161", "Applied Calculus II", ""},
		"ams210": {"AMS 210", "Applied Linear Algebra", ""},
		"ams261": {"AMS 261", "Calculus III", ""},
		"phy131": {"PHY 131", "Classical Physics I", ""},
		"cse113": {"CSE 113", "Foundations of Computer Science I", ""},
		"cse213": {"CSE 213", "Foundations of Computer Science II", ""},
		"psy103": {"PSY 103", "Introduction to Psychology", ""},
	}
	counts := map[string]int{"ams151": 25, "ams161": 26, "ams210": 30, "ams261": 45, "phy131": 51, "cse113": 32, "cse213": 34, "psy103": 22}
	total := 0
	for name, s := range sheets {
		if s.Course != want[name][0] {
			t.Errorf("%s: course = %q, want %q", name, s.Course, want[name][0])
		}
		if s.Title != want[name][1] {
			t.Errorf("%s: title = %q, want %q", name, s.Title, want[name][1])
		}
		if len(s.Formulas) != counts[name] {
			t.Errorf("%s: %d formulas, want %d", name, len(s.Formulas), counts[name])
		}
		total += len(s.Formulas)
	}
	if total != 265 {
		t.Errorf("total formulas = %d, want 265", total)
	}
}

func TestLoadSheetByNameUnknown(t *testing.T) {
	if _, err := LoadSheetByName("nope101"); err == nil {
		t.Error("expected error for unknown sheet")
	}
}

func TestByID(t *testing.T) {
	s, err := LoadSheetByName("phy131")
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.ByID("newton2")
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "Newton's second law" {
		t.Errorf("unexpected name: %q", f.Name)
	}
	if _, err := s.ByID("bogus"); err == nil {
		t.Error("expected error for unknown id")
	}
}

func TestIDsInOrder(t *testing.T) {
	s, _ := LoadSheetByName("ams261")
	ids := s.IDs()
	if len(ids) != 45 || ids[0] != s.Formulas[0].ID {
		t.Error("IDs() should follow sheet order")
	}
}

func TestTopicsOrder(t *testing.T) {
	s, _ := LoadSheetByName("psy103")
	topics := s.Topics()
	if len(topics) == 0 {
		t.Fatal("no topics")
	}
	if topics[0] != "Descriptive statistics" {
		t.Errorf("first topic = %q", topics[0])
	}
	// no duplicates, covers all formulas
	seen := map[string]bool{}
	for _, f := range s.Formulas {
		seen[f.Topic] = true
	}
	if len(topics) != len(seen) {
		t.Error("topics mismatch")
	}
}

func TestDuplicateIDRejected(t *testing.T) {
	data := []byte("course: X\ntitle: Y\nformulas:\n  - {id: a, name: A, latex: x}\n  - {id: a, name: B, latex: y}\n")
	if _, err := Load(data); err == nil {
		t.Error("expected duplicate id error")
	}
}

func TestSelfReferenceRejected(t *testing.T) {
	data := []byte("course: X\ntitle: Y\nformulas:\n  - {id: a, name: A, latex: x, derives_from: [a]}\n")
	if _, err := Load(data); err == nil {
		t.Error("expected self-reference error")
	}
}

func TestUnknownReferenceRejected(t *testing.T) {
	data := []byte("course: X\ntitle: Y\nformulas:\n  - {id: a, name: A, latex: x, derives_from: [ghost]}\n")
	if _, err := Load(data); err == nil {
		t.Error("expected unknown-reference error")
	}
}

func TestBadYAMLRejected(t *testing.T) {
	if _, err := Load([]byte(":\n: bad: [" )); err == nil {
		t.Error("expected YAML parse error")
	}
}

func TestDefaultsApplied(t *testing.T) {
	data := []byte("formulas:\n  - {id: a, name: A, latex: x}\n")
	s, err := Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.Formulas[0].Topic != "General" {
		t.Errorf("default topic = %q", s.Formulas[0].Topic)
	}
	if len(s.Formulas[0].DerivesFrom) != 0 {
		t.Error("default derives_from should be empty")
	}
}
