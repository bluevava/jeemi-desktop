//go:build bindings

package desktop

// Wails runs a temporary bindings binary on the build machine. It only needs
// the bound method set, so avoid resolving, creating, reading, or migrating any
// real user data in this mode.
func assembleApplication(resources Resources) (*App, string, error) {
	return &App{}, "", nil
}
