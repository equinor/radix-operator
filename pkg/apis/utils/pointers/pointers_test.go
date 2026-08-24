package pointers_test

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils/pointers"
	"github.com/stretchr/testify/assert"
)

func Test_Val(t *testing.T) {
	var p = new(1337)
	v := pointers.Val(p)

	assert.Equal(t, 1337, v)
	assert.Equal(t, 0, pointers.Val[int](nil))
	assert.Equal(t, "", pointers.Val[string](nil))
	assert.Equal(t, false, pointers.Val[bool](nil))
}

type i interface {
	getValue() int
}
type v struct{}

func (obj *v) getValue() int {
	return 1
}

func Test_IsNil(t *testing.T) {
	type args struct {
		obj any
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "not nil",
			args: args{obj: &v{}},
			want: false,
		},
		{
			name: "not pointer",
			args: args{obj: 1},
			want: false,
		},
		{
			name: "is nil",
			args: args{obj: nil},
			want: true,
		},
		{
			name: "is nil value in the interface",
			args: args{obj: func(obj i) i { return obj }(nil)},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, pointers.IsNil(tt.args.obj), "IsNil(%v)", tt.args.obj)
		})
	}
}
