package scan

import "strings"

type Provenance int

const (
	Unmanaged Provenance = iota
	Managed
	AppStore
	System
)

func (p Provenance) String() string {
	return [...]string{"unmanaged", "managed", "app-store", "system"}[p]
}

// Classify determines an app's provenance. brewOwned maps lowercased
// .app artifact names to the owning cask token.
func Classify(app App, brewOwned map[string]string) Provenance {
	switch {
	case strings.HasPrefix(app.BundleID, "com.apple."):
		return System
	case app.MASReceipt:
		return AppStore
	default:
		if _, ok := brewOwned[strings.ToLower(app.Name)]; ok {
			return Managed
		}
		return Unmanaged
	}
}
