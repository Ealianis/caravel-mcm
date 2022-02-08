package errors

import "errors"

func WrongUrlOrCredentials() error {
	return errors.New("wrong URL or Credentials provided")
}
