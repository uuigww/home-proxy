package i18n

import (
	"strings"
	"testing"
)

func TestNewLoadsLocales(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	langs := b.Langs()
	if len(langs) < 2 {
		t.Fatalf("expected at least ru+en, got %v", langs)
	}
	foundRu, foundEn := false, false
	for _, l := range langs {
		if l == "ru" {
			foundRu = true
		}
		if l == "en" {
			foundEn = true
		}
	}
	if !foundRu || !foundEn {
		t.Fatalf("expected ru and en, got %v", langs)
	}
}

func TestTRussianLookup(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	got := b.T("ru", "menu.users")
	if got == "" || got == "menu.users" {
		t.Fatalf("expected translated users label, got %q", got)
	}
	// Russian string should be cyrillic.
	if !containsCyrillic(got) {
		t.Fatalf("expected cyrillic text, got %q", got)
	}
}

func TestTFallbackToEnglish(t *testing.T) {
	// Build a hand-rolled bundle to force the fallback path: the "ru" map
	// is missing the key, but "en" has it.
	b := &Bundle{langs: map[string]map[string]string{
		"ru": {"menu.home": "Меню"},
		"en": {"only.english": "English Only"},
	}}
	if got := b.T("ru", "only.english"); got != "English Only" {
		t.Fatalf("expected English fallback, got %q", got)
	}
	// ...but if neither has the key, we return the key itself.
	if got := b.T("ru", "missing.everywhere"); got != "missing.everywhere" {
		t.Fatalf("expected raw key, got %q", got)
	}
}

func TestTMissingKeyReturnsKey(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	const missing = "this.key.does.not.exist"
	if got := b.T("ru", missing); got != missing {
		t.Fatalf("expected raw key back, got %q", got)
	}
}

func TestTSprintfArgs(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	got := b.T("ru", "menu.summary", 3, "12.3 GB")
	if !strings.Contains(got, "3") || !strings.Contains(got, "12.3 GB") {
		t.Fatalf("expected args substituted, got %q", got)
	}
}

func TestHas(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if !b.Has("ru", "menu.users") {
		t.Fatalf("expected Has(ru, menu.users) = true")
	}
	if b.Has("ru", "does.not.exist") {
		t.Fatalf("expected Has(ru, does.not.exist) = false")
	}
}

func TestNormalise(t *testing.T) {
	cases := map[string]string{
		"":   "ru",
		"ru": "ru",
		"RU": "ru",
		"en": "en",
		"EN": "en",
		"fr": "ru",
	}
	for in, want := range cases {
		if got := Normalise(in); got != want {
			t.Errorf("Normalise(%q) = %q, want %q", in, got, want)
		}
	}
}

func containsCyrillic(s string) bool {
	for _, r := range s {
		if r >= 0x0400 && r <= 0x04FF {
			return true
		}
	}
	return false
}
