package test

import (
	"fmt"
	"reflect"

	event "github.com/equinor/radix-operator/api-server/api/events"
	"go.uber.org/mock/gomock"
)

type namespaceFuncMatcher struct {
	f event.NamespaceFunc
}

func (m namespaceFuncMatcher) Matches(arg interface{}) bool {
	argv := reflect.ValueOf(arg)
	fv := reflect.ValueOf(m.f)

	// Equal if both expected and actual is nil
	if argv.IsNil() && fv.IsNil() {
		return true
	}

	// If expected and actual IsNil() is different they are not equal
	if argv.IsNil() != fv.IsNil() {
		return false
	}

	// Not equal if actual or expected is not a function
	if argv.Kind() != reflect.Func || fv.Kind() != reflect.Func {
		return false
	}

	// equal if functions return same value
	return m.f() == arg.(event.NamespaceFunc)()
}

func (m namespaceFuncMatcher) String() string {
	fv := reflect.ValueOf(m.f)

	if fv.IsNil() {
		return "<nil>"
	}

	return fmt.Sprintf("%v", fv)
}

// EqualsNamespaceFunc compares namespace function type and value returned
func EqualsNamespaceFunc(f event.NamespaceFunc) gomock.Matcher {
	return namespaceFuncMatcher{f: f}
}
