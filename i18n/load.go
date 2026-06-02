package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// LoadFromFS reads every *.json file directly under dir in fsys and
// returns an I18n instance. File names (minus .json) become locale
// codes. Consumers typically pass an embed.FS so locale files ship
// inside the binary:
//
//	//go:embed locales/*.json
//	var localesFS embed.FS
//
//	i18nInstance, err := i18n.LoadFromFS(localesFS, "locales", "en", "en")
func LoadFromFS(fsys fs.FS, dir, defaultLang, fallbackLang string) (*I18n, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("i18n: read dir %q: %w", dir, err)
	}
	translations := make(map[string]Translation, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		locale := strings.TrimSuffix(entry.Name(), ".json")
		data, err := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join(dir, entry.Name())))
		if err != nil {
			return nil, fmt.Errorf("i18n: read %s: %w", entry.Name(), err)
		}
		var t Translation
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, fmt.Errorf("i18n: parse %s: %w", entry.Name(), err)
		}
		translations[locale] = t
	}
	return NewI18n(defaultLang, fallbackLang, translations), nil
}

// LoadFromDir is the os.DirFS equivalent of LoadFromFS — useful for
// loading translations at runtime from disk during development.
func LoadFromDir(dir, defaultLang, fallbackLang string) (*I18n, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("i18n: abs %q: %w", dir, err)
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("i18n: stat %q: %w", abs, err)
	}
	return LoadFromFS(os.DirFS(abs), ".", defaultLang, fallbackLang)
}
