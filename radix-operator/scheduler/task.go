package scheduler

type Task interface {
	Run() error
}
