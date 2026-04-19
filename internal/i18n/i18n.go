// Package i18n is a tiny, flat-key message bundle used by the Telegram bot.
//
// Locales are embedded via go:embed at build time; they are parsed once on
// startup into a map[lang]map[flatKey]string. Nested TOML tables are
// flattened into dot-keys so callers can ignore hierarchy:
//
//	[menu]
//	users = "Users"
//	# flattens to key "menu.users"
//
// Only "ru" and "en" are recognised. Anything else is treated as "ru" by
// Normalise and falls back to English on a missing key.
package i18n

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

//go:embed locales/*.toml
var localesFS embed.FS

// Bundle is an immutable collection of translated strings keyed by language
// code and dot-path.
type Bundle struct {
	langs map[string]map[string]string
}

// New parses every embedded locale file and returns a ready-to-use Bundle.
//
// A locale file named `xx.toml` contributes a language "xx"; nested TOML
// tables are flattened with '.' separators.
func New() (*Bundle, error) {
	entries, err := fs.ReadDir(localesFS, "locales")
	if err != nil {
		return nil, fmt.Errorf("i18n: read locales dir: %w", err)
	}
	b := &Bundle{langs: make(map[string]map[string]string, len(entries))}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		lang := strings.TrimSuffix(e.Name(), ".toml")
		data, err := fs.ReadFile(localesFS, "locales/"+e.Name())
		if err != nil {
			return nil, fmt.Errorf("i18n: read %s: %w", e.Name(), err)
		}
		var raw map[string]any
		if err := toml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("i18n: parse %s: %w", e.Name(), err)
		}
		flat := make(map[string]string, 64)
		flatten("", raw, flat)
		b.langs[lang] = flat
	}
	if len(b.langs) == 0 {
		return nil, fmt.Errorf("i18n: no locales loaded")
	}
	return b, nil
}

// T returns the translated string for (lang, key). If the key is missing in
// the requested language, the English translation is used; if that is also
// missing, the key itself is returned so the UI still renders.
//
// When args is non-empty the resolved template is passed through
// fmt.Sprintf.
func (b *Bundle) T(lang, key string, args ...any) string {
	if b == nil {
		return key
	}
	lang = Normalise(lang)
	if m, ok := b.langs[lang]; ok {
		if s, ok := m[key]; ok {
			return format(s, args)
		}
	}
	if lang != "en" {
		if m, ok := b.langs["en"]; ok {
			if s, ok := m[key]; ok {
				return format(s, args)
			}
		}
	}
	return key
}

// Has reports whether the bundle has an entry for (lang, key). Fallback
// languages are NOT consulted — callers use Has to decide whether to render
// an optional section.
func (b *Bundle) Has(lang, key string) bool {
	if b == nil {
		return false
	}
	m, ok := b.langs[Normalise(lang)]
	if !ok {
		return false
	}
	_, ok = m[key]
	return ok
}

// Langs returns the sorted list of language codes present in the bundle.
func (b *Bundle) Langs() []string {
	if b == nil {
		return nil
	}
	out := make([]string, 0, len(b.langs))
	for k := range b.langs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Normalise collapses any unknown or empty language code to "ru". "en" is
// returned as-is.
func Normalise(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "en":
		return "en"
	default:
		return "ru"
	}
}

// flatten walks src and writes every leaf value into dst keyed by its
// dot-path. Non-string leaves are rendered via fmt.Sprint so TOML ints and
// bools can still ship through Bundle.T (mostly useful for `meta.*`).
func flatten(prefix string, src map[string]any, dst map[string]string) {
	for k, v := range src {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch vv := v.(type) {
		case map[string]any:
			flatten(key, vv, dst)
		case string:
			dst[key] = vv
		default:
			dst[key] = fmt.Sprint(vv)
		}
	}
}

// format applies fmt.Sprintf only when args were provided — avoids turning
// `%%` escapes into surprises for callers that pass no args.
func format(tmpl string, args []any) string {
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}
