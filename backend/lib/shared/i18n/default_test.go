package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMustDefaultCatalogViaEnv(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "cn", "errors.json"), `{"X": "x-cn"}`)
	writeJSON(t, filepath.Join(dir, "en", "errors.json"), `{"X": "x-en"}`)

	// Point the singleton loader at our temp dir.
	t.Setenv("IDCD_I18N_MESSAGES_PATH", dir)
	resetDefaultCatalogForTests()
	t.Cleanup(resetDefaultCatalogForTests)
	t.Cleanup(func() { _ = os.Unsetenv("IDCD_I18N_MESSAGES_PATH") })

	cat := MustDefaultCatalog()
	if got := cat.T("en", "errors.X", nil); got != "x-en" {
		t.Errorf("singleton catalog T = %q want x-en", got)
	}
	if got := cat.T("cn", "errors.X", nil); got != "x-cn" {
		t.Errorf("singleton catalog T cn = %q want x-cn", got)
	}
}

func TestLocateDefaultMessagesEnvOverride(t *testing.T) {
	t.Setenv("IDCD_I18N_MESSAGES_PATH", "/explicit/path/messages")
	got, err := locateDefaultMessages()
	if err != nil {
		t.Fatalf("locate: %v", err)
	}
	if got != "/explicit/path/messages" {
		t.Errorf("env override ignored; got %q", got)
	}
}
