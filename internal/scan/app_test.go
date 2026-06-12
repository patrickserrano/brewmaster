package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func writeApp(t *testing.T, dir, name, bundleID, version string, mas bool) string {
	t.Helper()
	app := filepath.Join(dir, name)
	contents := filepath.Join(app, "Contents")
	if err := os.MkdirAll(contents, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleIdentifier</key><string>` + bundleID + `</string>
  <key>CFBundleShortVersionString</key><string>` + version + `</string>
  <key>CFBundleName</key><string>Fixture</string>
  <key>CFBundleExecutable</key><string>FixtureBin</string>
</dict></plist>`
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	if mas {
		receiptDir := filepath.Join(contents, "_MASReceipt")
		if err := os.MkdirAll(receiptDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(receiptDir, "receipt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func TestReadApp(t *testing.T) {
	dir := t.TempDir()
	path := writeApp(t, dir, "Slack.app", "com.tinyspeck.slackmacgap", "4.39.0", false)

	app, err := ReadApp(path)
	if err != nil {
		t.Fatalf("ReadApp: %v", err)
	}
	if app.Name != "Slack.app" || app.BundleID != "com.tinyspeck.slackmacgap" ||
		app.Version != "4.39.0" || app.MASReceipt {
		t.Errorf("got %+v", app)
	}
	if app.Executable != "FixtureBin" {
		t.Errorf("Executable = %q, want %q", app.Executable, "FixtureBin")
	}
}

func TestScanDirsFindsAppsOneLevelDeep(t *testing.T) {
	dir := t.TempDir()
	writeApp(t, dir, "Slack.app", "com.tinyspeck.slackmacgap", "4.39.0", false)
	writeApp(t, dir, "Things3.app", "com.culturedcode.ThingsMac", "3.20", true)
	// Non-app dir and nested app must be ignored.
	os.MkdirAll(filepath.Join(dir, "NotAnApp"), 0o755)
	nested := filepath.Join(dir, "SomeFolder")
	os.MkdirAll(nested, 0o755)
	writeApp(t, nested, "Hidden.app", "com.x.hidden", "1.0", false)

	apps, err := ScanDirs([]string{dir, filepath.Join(dir, "does-not-exist")})
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 {
		t.Fatalf("want 2 apps, got %d: %+v", len(apps), apps)
	}
}

func TestReadAppDetectsMASReceipt(t *testing.T) {
	dir := t.TempDir()
	path := writeApp(t, dir, "Things3.app", "com.culturedcode.ThingsMac", "3.20", true)

	app, err := ReadApp(path)
	if err != nil {
		t.Fatal(err)
	}
	if !app.MASReceipt {
		t.Error("expected MASReceipt=true")
	}
}
