package http

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
)

// Error Representation of errors in the API. These are divided into a small
// number of categories, essentially distinguished by whose fault the
// error is; i.e., is this error:
//   - a transient problem with the service, so worth trying again?
//   - not going to work until the user takes some other action, e.g., updating config?
type Error struct {
	Type Type
	// a message that can be printed out for the user
	Message string `json:"message"`
	// the underlying error that can be e.g., logged for developers to look at
	Err error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}

	return e.Message
}

// Type Type of error
type Type string

const (
	// Server The operation looked fine on paper, but something went wrong
	Server Type = "server"
	// Missing The thing you mentioned, whatever it is, just doesn't exist
	Missing = "missing"
	// User The operation was well-formed, but you asked for something that
	// can't happen at present (e.g., because you've not supplied some
	// config yet)
	User = "user"
	// Forbidden The operation is not allowed for the current authenticated user
	Forbidden = "forbidden"
)

// MarshalJSON Writes error as json
func (e *Error) MarshalJSON() ([]byte, error) {
	var errMsg string
	if e.Err != nil {
		errMsg = e.Err.Error()
	}
	jsonable := &struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Err     string `json:"error,omitempty"`
	}{
		Type:    string(e.Type),
		Message: e.Message,
		Err:     errMsg,
	}
	return json.Marshal(jsonable)
}

// UnmarshalJSON Parses json
func (e *Error) UnmarshalJSON(data []byte) error {
	jsonable := &struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Err     string `json:"error,omitempty"`
	}{}
	if err := json.Unmarshal(data, &jsonable); err != nil {
		return err
	}
	e.Type = Type(jsonable.Type)
	e.Message = jsonable.Message
	if jsonable.Err != "" {
		e.Err = errors.New(jsonable.Err)
	}
	return nil
}

// UnexpectedError any unexpected error
func UnexpectedError(message string, underlyingError error) error {
	return &Error{
		Type:    Server,
		Err:     underlyingError,
		Message: message,
	}
}

// TypeMissingError indication of underlying type missing
func TypeMissingError(message string, underlyingError error) error {
	return &Error{
		Type:    Missing,
		Err:     underlyingError,
		Message: message,
	}
}

// ValidationError Used for indication of validation errors
func ValidationError(kind, message string) error {
	return &Error{
		Type:    User,
		Err:     fmt.Errorf("%s failed validation", kind),
		Message: message,
	}
}

// ForbiddenError forbidden error
func ForbiddenError(message string) error {
	return &Error{
		Type:    Forbidden,
		Message: message,
	}
}

// NotFoundError No found error
func NotFoundError(message string) error {
	return &Error{
		Type:    Missing,
		Message: message,
	}
}

// ApplicationNotFoundError indication that application was not found. Can also mean a user does not have access to the application.
func ApplicationNotFoundError(message string, underlyingError error) error {
	return &Error{
		Type:    Missing,
		Err:     underlyingError,
		Message: message,
	}
}

// CoverAllError Cover all other errors for requester type Type
func CoverAllError(err error, requesterType Type) *Error {
	return &Error{
		Type:    requesterType,
		Err:     err,
		Message: `Error: ` + err.Error(),
	}
}
