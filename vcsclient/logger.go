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
}

func (el EmptyLogger) Info(_ ...interface{}) {
}

func (el EmptyLogger) Warn(_ ...interface{}) {
}

func (el EmptyLogger) Error(_ ...interface{}) {
}

func (el EmptyLogger) Output(_ ...interface{}) {
}
