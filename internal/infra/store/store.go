package store

// SessionStore abstracts session persistence.
// v1: in-memory (Registry itself serves as the store).
// v1.5: SQLite implementation.
type SessionStore interface {
	SaveSession(key string, data []byte) error
	LoadSession(key string) ([]byte, error)
	ListSessions() ([][]byte, error)
	RemoveSession(key string) error
}
