package executor

var executorFactory = make(map[string]CreateExecutor)

type CreateExecutor func(fn SyncResultFn) (Executor, error)

// TODO 注册时同时写到mysql中，scheduler 可以提前判断任务类型是否支持.
func RegisteFactory(taskType string, ce CreateExecutor) {
	executorFactory[taskType] = ce
}

func GetFactory(taskType string) (CreateExecutor, bool) {
	e, ok := executorFactory[taskType]
	return e, ok
}
