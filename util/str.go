package util

func SplitOnce(s string, c byte) (left string, right string) {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			left = s[0:i]
			right = s[i+1:]
			return
		}
	}
	left = s
	return
}

// Sanitize the given string by turning whitespaces other than `\t` and ` ` to ` `.
//
// Prevent (log) injection by sanitizing input from users.
func SanitizeWhite(s string) string {
	bs := []byte(s)
	for i, b := range bs {
		if b == '\n' || b == '\r' {
			bs[i] = ' '
		}
	}
	return string(bs)
}
