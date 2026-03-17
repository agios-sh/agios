package cmd

type cmdError struct {
	msg   string
	code  string
	cause error
}

func (e *cmdError) Error() string { return e.msg }

func (e *cmdError) Unwrap() error { return e.cause }
