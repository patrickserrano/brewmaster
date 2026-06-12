package scan

import (
	"os"
	"path/filepath"

	"howett.net/plist"
)

type App struct {
	Path        string
	Name        string // bundle dir name, e.g. "Slack.app"
	BundleID    string
	Version     string
	DisplayName string
	Executable  string // CFBundleExecutable, used for running-app detection
	MASReceipt  bool
}

type infoPlist struct {
	BundleID    string `plist:"CFBundleIdentifier"`
	Version     string `plist:"CFBundleShortVersionString"`
	DisplayName string `plist:"CFBundleName"`
	Executable  string `plist:"CFBundleExecutable"`
}

func ReadApp(path string) (App, error) {
	data, err := os.ReadFile(filepath.Join(path, "Contents", "Info.plist"))
	if err != nil {
		return App{}, err
	}
	var info infoPlist
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return App{}, err
	}
	_, receiptErr := os.Stat(filepath.Join(path, "Contents", "_MASReceipt", "receipt"))
	return App{
		Path:        path,
		Name:        filepath.Base(path),
		BundleID:    info.BundleID,
		Version:     info.Version,
		DisplayName: info.DisplayName,
		Executable:  info.Executable,
		MASReceipt:  receiptErr == nil,
	}, nil
}
