package core

type NaruModule interface {
	Name() string

	Load() error
	Unload() error
}
