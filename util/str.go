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
