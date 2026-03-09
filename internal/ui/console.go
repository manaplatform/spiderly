package ui

// Console is kept for backward compatibility
// The main UI is now the web dashboard
type Console struct{}

func NewConsole() *Console {
	return &Console{}
}
