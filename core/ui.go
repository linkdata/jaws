package core

// UI defines the required methods on JaWS UI objects.
// In addition, all UI objects must be comparable so they can be used as map keys.
type UI interface {
	Renderer
	Updater
}
