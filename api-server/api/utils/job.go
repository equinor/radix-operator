package utils

import (
	"time"
)

type Job interface {
	GetCreated() *time.Time
	GetStarted() *time.Time
	GetEnded() *time.Time
	GetStatus() string
}

// IsBefore Checks that job-j is before job-i
func IsBefore(j, i Job) bool {
	jCreated := j.GetCreated()
	iCreated := i.GetCreated()
	jStarted := j.GetStarted()
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
