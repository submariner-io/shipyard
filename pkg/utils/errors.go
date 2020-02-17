package utils

import "fmt"

func ErrorIfOccurrs(err error, message string, args ...interface{}) error {
	if err != nil {
		return fmt.Errorf(message, args...)
	}

	return nil
}
