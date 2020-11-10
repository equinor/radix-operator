package branch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type branchNameTest struct {
	s     string
	valid bool
}

func TestIsValidName(t *testing.T) {

	tests := []branchNameTest{
		{"main?", false},
		{"ma?in", false},
		{"?main", false},
		{"main ", false},
		{"ma in", false},
		{" main", false},
		{"main\\", false},
		{"ma\\in", false},
		{"\\main", false},
		{"main^", false},
		{"ma^in", false},
		{"^main", false},
		{"main~", false},
		{"ma~in", false},
		{"~main", false},
		{"main:", false},
		{"ma:in", false},
		{":main", false},
		{"main*", false},
		{"ma*in", false},
		{"*main", false},
		{"main[", false},
		{"ma[in", false},
		{"[main", false},
		{"main/.feat", false},
		{"/main", false},
		{"main/", false},
		{"ma//in", false},
		{string([]byte{0x40, 0x41, 0x41, 0x1F}), false},
		{string([]byte{0x40, 0x41, 0x41, 0x7F}), false},
		{"main..", false},
		{"ma..in", false},
		{"..main", false},
		{"ma@{in", false},
		{"main.lock", false},
		{"main.lock/feat", false},
		{"@", false},
		{"", false},
		{"main", true},
		{"main/feature", true},
		{"main/feat.ure", true},
		{"main/feature.", true},
		{"main/@feature", true},
	}

	for _, test := range tests {
		t.Run(test.s, func(t *testing.T) {
			assert.Equal(t, test.valid, IsValidName(test.s))
		})
	}
}
