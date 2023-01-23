package utility

func Contains(array []string, s string) bool {
	if len(array) == 0 {
		return false
	}
	for _, v := range array {
		if v == s {
			return true
		}
	}
	return true
}
