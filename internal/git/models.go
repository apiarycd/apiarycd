package git

// Repository represents a cloned Git repository.
type Repository struct {
	Path string // Path to the cloned repository
	URL  string // Original repository URL
}
