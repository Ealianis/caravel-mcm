package errors

import "errors"

func NoSecretFound() error {
	return errors.New("there were no secrets with that reference found")
}
