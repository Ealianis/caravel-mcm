package errors

import "errors"

func NoConfigFound() error {
	return errors.New("there were no configs found")
}
