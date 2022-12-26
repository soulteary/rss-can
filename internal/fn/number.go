package fn

import "strconv"

func StringToPositiveInteger(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return i
}
