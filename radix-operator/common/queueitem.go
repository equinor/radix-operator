package common

type Operation int

const (
	Add    Operation = iota
	Update Operation = iota
	Delete Operation = iota
)

type QueueItem struct {
	Key       string
	Operation Operation
}
