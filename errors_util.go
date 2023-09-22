package cofaas

import "github.com/go-errors/errors"

func FormatError(err error) string {
	if err, ok := err.(*errors.Error); err != nil && ok {
		return err.Error() + "\n\n" + err.ErrorStack()
	} else if err != nil {
		return err.Error()
	} else {
		return ""
	}
}
