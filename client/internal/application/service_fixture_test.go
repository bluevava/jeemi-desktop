package application

// Existing unit fixtures inject one temporary root. Workspace integration tests
// exercise PrepareConfigurationWorkspace and the production NewService separately.
func newTestService(root string) (*Service, error) { return newService(root, root) }
