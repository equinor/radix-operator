package secret

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFixedStringRunes_ReturnsFixedStringCharacters(t *testing.T) {
	expected := "aaaaa"
	actual := FixedStringRunes(5, 'a')
	assert.Equal(t, expected, actual)
}

func TestObfuscate_ReturnsObfuscatedString(t *testing.T) {
	data := "abcdefghijklm"
	expected := "abcxxxxxxjklm"
	actual := Obfuscate(data, 4, 6, 'x')
	assert.Equal(t, expected, actual)
}
