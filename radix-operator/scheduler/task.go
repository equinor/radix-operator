package scheduler

type Task interface {
	Start()
	Stop()
}
