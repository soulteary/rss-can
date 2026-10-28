package jssdk

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ConvertAgoToUnix(date string) (time.Time, error) {
	return convertDateContext(context.Background(), "ConvertAgoToUnix", date)
}

func ConvertStrToUnix(str string) (time.Time, error) {
	return ConvertStrToUnixContext(context.Background(), str)
}

func ConvertStrToUnixContext(ctx context.Context, str string) (time.Time, error) {
	return convertDateContext(ctx, "ConvertStrToUnix", str)
}

func convertDateContext(ctx context.Context, function, input string) (time.Time, error) {
	if err := ctx.Err(); err != nil {
		return time.Time{}, err
	}
	if strings.TrimSpace(input) == "" {
		return time.Time{}, fmt.Errorf("date must not be empty")
	}
	unixStr, err := RunCodeContext(ctx, TPL_DATE_JS, function+"("+strconv.Quote(input)+")", "convert.js")
	if err != nil {
		return time.Time{}, err
	}
	return parseUnixDateResult(unixStr)
}

// Unix timestamps are signed: dates before 1970 are valid. A malformed JS
// result is a conversion failure, never a fabricated epoch/sentinel date.
func parseUnixDateResult(result string) (time.Time, error) {
	seconds, err := strconv.ParseInt(result, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date timestamp %q: %w", result, err)
	}
	return time.Unix(seconds, 0), nil
}
