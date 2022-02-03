package errors

import "errors"

func ClientNotCreated() error {
	return errors.New("client was unable to be created")
}
