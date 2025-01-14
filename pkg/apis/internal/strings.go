package internal

// EqualTillPostfix Compares two strings till the postfix
func EqualTillPostfix(value1, value2 string, postfixLength int) bool {
	if len(value1) < postfixLength || len(value2) < postfixLength {
		return false
	}
	return value1[:len(value1)-postfixLength] == value2[:len(value2)-postfixLength]
}
