package gopipe

import "errors"

func FirstError(errs <-chan error) error {
	for e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

func JoinErrors(errs <-chan error) error {
	var list []error
	for e := range errs {
		if e != nil {
			list = append(list, e)
		}
	}
	if len(list) == 0 {
		return nil
	}
	return errors.Join(list...)
}
