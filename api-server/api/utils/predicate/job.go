package predicate

import (
	"time"
)

type Job interface {
	GetCreated() *time.Time
	GetStarted() *time.Time
	GetEnded() *time.Time
}

// IsJobBefore checks if job j1 is before job j2
func IsJobBefore(j1, i Job) bool {
	jCreated := j1.GetCreated()
	iCreated := i.GetCreated()
	jStarted := j1.GetStarted()
	iStarted := i.GetStarted()

	if jCreated == nil {
		return false
	}

	if iCreated == nil {
		return true
	}

	if iStarted != nil && jStarted != nil && jStarted.Before(*iStarted) {
		return true
	}

	if jCreated.Equal(*iCreated) {
		return false
	}

	return jCreated.Before(*iCreated)
}
