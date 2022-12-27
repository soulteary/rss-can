package fn

import (
	"time"
)

func I2T(n int) time.Duration {
	return time.Duration(n)
}

func ExpireBySecond(n int) time.Duration {
	return I2T(n) * time.Second
}
