package vcsclient

type Log interface {
	Debug(a ...interface{})
	Info(a ...interface{})
	Warn(a ...interface{})
	Error(a ...interface{})
	Output(a ...interface{})
}

type EmptyLogger struct{}

func (el EmptyLogger) Debug(_ ...interface{}) {
	return
}

func (el EmptyLogger) Info(_ ...interface{}) {
	return
}

func (el EmptyLogger) Warn(a ...interface{}) {
	return
}

func (el EmptyLogger) Error(_ ...interface{}) {
	return
}

func (el EmptyLogger) Output(_ ...interface{}) {
	return
}
