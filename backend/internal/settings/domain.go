package settings

import "errors"

type Settings struct {
	PrimaryCurrency string
	TimeZone        string
}

var ErrNotFound = errors.New("settings: not found")

func Defaults() Settings {
	return Settings{PrimaryCurrency: "RSD", TimeZone: "Europe/Belgrade"}
}
