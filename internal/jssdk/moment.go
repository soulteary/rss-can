package jssdk

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/soulteary/RSS-Can/internal/fn"
)

func ConvertAgoToUnix(date string) (time.Time, error) {
	unixStr, err := RunCode(TPL_DATE_JS, "ConvertAgoToUnix("+strconv.Quote(date)+")", "convert.js")
	if err != nil {
		return time.Now(), err
	}

	timeUnix := time.Unix(int64(fn.StringToPositiveInteger(fmt.Sprint(unixStr))), 0)
	return timeUnix, nil
}

func ConvertStrToUnix(str string) (time.Time, error) {
	return ConvertStrToUnixContext(context.Background(), str)
}

func ConvertStrToUnixContext(ctx context.Context, str string) (time.Time, error) {
	unixStr, err := RunCodeContext(ctx, TPL_DATE_JS, "ConvertStrToUnix("+strconv.Quote(str)+")", "convert.js")
	if err != nil {
		return time.Now(), err
	}

	timeUnix := time.Unix(int64(fn.StringToPositiveInteger(fmt.Sprint(unixStr))), 0)
	return timeUnix, nil
}
