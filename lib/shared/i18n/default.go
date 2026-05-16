package i18n

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	defaultCatalog     *Catalog
	defaultCatalogOnce sync.Once
	defaultCatalogErr  error
)

// MustDefaultCatalog returns the process-wide message catalog loaded from
// lib/shared/i18n/messages/. It searches upward from the cwd in the same
// way MustDefault searches for config/locales.json, and panics if loading
// fails (mirrors MustDefault's "treat config loss as a startup error"
// behaviour).
//
// The IDCD_I18N_MESSAGES_PATH env var can override the search.
func MustDefaultCatalog() *Catalog {
	defaultCatalogOnce.Do(func() {
		path, err := locateDefaultMessages()
		if err != nil {
			defaultCatalogErr = err
			return
		}
		defaultCatalog, defaultCatalogErr = Load(path, MustDefault())
	})
	if defaultCatalogErr != nil {
		panic(defaultCatalogErr)
	}
	return defaultCatalog
}

// resetDefaultCatalogForTests clears the singleton — exported only via the
// test helper file. Putting it here keeps the sync.Once + state in one place.
func resetDefaultCatalogForTests() {
	defaultCatalog = nil
	defaultCatalogErr = nil
	defaultCatalogOnce = sync.Once{}
}

func locateDefaultMessages() (string, error) {
	if envPath := os.Getenv("IDCD_I18N_MESSAGES_PATH"); envPath != "" {
		return envPath, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for range 8 {
		candidate := filepath.Join(dir, "lib", "shared", "i18n", "messages")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("i18n: lib/shared/i18n/messages/ not found above %s; set IDCD_I18N_MESSAGES_PATH", cwd)
}
