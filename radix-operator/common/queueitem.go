package common

// Operation Type
type Operation int

// Supported Operations
const (
	Add    Operation = iota
	Update Operation = iota
	Delete Operation = iota
)

// QueueItem Element to put on worker queue
type QueueItem struct {
	Key       string
	OldObject interface{}
	Operation Operation
}
