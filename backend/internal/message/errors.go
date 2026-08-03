package message

import "fmt"

type PolicyError struct {
	Code    string
	Message string
}

func (e *PolicyError) Error() string {
	return e.Message
}

func policyError(code, message string) error {
	return &PolicyError{Code: code, Message: message}
}

func internalError(operation string, err error) error {
	return fmt.Errorf("%s: %w", operation, err)
}
