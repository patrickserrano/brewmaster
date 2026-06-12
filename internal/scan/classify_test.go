package scan

import "testing"

func TestClassify(t *testing.T) {
	brewOwned := map[string]string{"visual studio code.app": "visual-studio-code"}
	cases := []struct {
		name string
		app  App
		want Provenance
	}{
		{"brew-managed", App{Name: "Visual Studio Code.app", BundleID: "com.microsoft.VSCode"}, Managed},
		{"mas", App{Name: "Things3.app", BundleID: "com.culturedcode.ThingsMac", MASReceipt: true}, AppStore},
		{"apple system", App{Name: "Safari.app", BundleID: "com.apple.Safari"}, System},
		{"unmanaged", App{Name: "Slack.app", BundleID: "com.tinyspeck.slackmacgap"}, Unmanaged},
		// MAS receipt wins over brew ownership (shouldn't co-occur, but be deterministic).
		{"mas wins", App{Name: "Visual Studio Code.app", MASReceipt: true}, AppStore},
		// Real Apple MAS apps carry both signals; System must win.
		{"system wins over mas", App{Name: "Pages.app", BundleID: "com.apple.iWork.Pages", MASReceipt: true}, System},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Classify(c.app, brewOwned); got != c.want {
				t.Errorf("Classify(%s) = %v, want %v", c.app.Name, got, c.want)
			}
		})
	}
}
