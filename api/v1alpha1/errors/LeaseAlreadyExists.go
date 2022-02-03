package errors

import "errors"

func LeaseAlreadyExists(id string) error {
	return errors.New("this cluster is already in a fleet" + id)
}
