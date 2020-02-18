package errors

import "fmt"

func IfOccurs(err error, message string, args ...interface{}) error {
	if err != nil {
		return fmt.Errorf(message, args...)
	}

	return nil
}
