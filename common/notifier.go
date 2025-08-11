package common

// Provides functionality for notifiers to send messages about actions
// taken by the email scanner.
type Notifier interface {
	// Send a message using the notifier. The message will be a markdown string.
	Notify(markdownMessage string) error
}
