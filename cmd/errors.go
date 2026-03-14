package cmd

// cmdError is a structured error used by command implementations to carry
// both a user-facing message and an AIP error code.
type cmdError struct {
	msg   string
	code  string
	cause error
}

func (e *cmdError) Error() string { return e.msg }

func (e *cmdError) Unwrap() error { return e.cause }
