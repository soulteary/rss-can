package jssdk

import (
	"fmt"
	"time"

	"github.com/soulteary/RSS-Can/internal/fn"
	v8 "rogchap.com/v8go"
)

var DateFn = func() string {
	return fmt.Sprintf("%s\n%s", MomentJS, DateJS)
}()

func ConvertAgoToUnix(date string) (time.Time, error) {
	ctx := v8.NewContext()
	_, _, err := RunCodeInSandbox(ctx, DateFn, "moment.js")

	if err != nil {
		return time.Now(), err
	}

	unixStr, err := ctx.RunScript(`ConvertAgoToUnix("`+date+`")`, "convert.js")
	if err != nil {
		return time.Now(), err
	}

	timeUnix := time.Unix(int64(fn.StringToPositiveInteger(fmt.Sprint(unixStr))), 0)
	return timeUnix, nil
}

func ConvertStrToUnix(str string) (time.Time, error) {
	ctx := v8.NewContext()
	_, _, err := RunCodeInSandbox(ctx, DateFn, "moment.js")

	if err != nil {
		return time.Now(), err
	}

	unixStr, err := ctx.RunScript(`ConvertStrToUnix("`+str+`")`, "convert.js")
	if err != nil {
		return time.Now(), err
	}

	timeUnix := time.Unix(int64(fn.StringToPositiveInteger(fmt.Sprint(unixStr))), 0)
	return timeUnix, nil
}
