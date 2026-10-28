package jssdk

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDateContractPreservesSignedEpochAndTimeZones(t *testing.T) {
	for _, tc := range []struct {
		input   string
		seconds int64
	}{
		{"1970-01-01T00:00:00Z", 0}, {"1960-01-01T00:00:00Z", -315619200}, {"1969-12-31T23:59:59Z", -1},
		{"2026-10-29T08:00:00+08:00", 1793232000}, {"Thu, 29 Oct 2026 00:00:00 GMT", 1793232000},
	} {
		got, err := ConvertStrToUnix(tc.input)
		if err != nil || got.Unix() != tc.seconds {
			t.Fatalf("%q: got %d (%s), %v", tc.input, got.Unix(), got, err)
		}
	}
	for _, input := range []string{"2022年01月02日", "2022/1/2 03:04:05", "2022年1月2日 3:04:05"} {
		got, err := ConvertStrToUnix(input)
		expected := time.Date(2022, time.January, 2, 3, 4, 5, 0, time.Local)
		if input == "2022年01月02日" {
			expected = time.Date(2022, time.January, 2, 0, 0, 0, 0, time.Local)
		}
		if err != nil || !got.Equal(expected) {
			t.Fatalf("%q: %s %v", input, got, err)
		}
	}
}

func TestDateContractRejectsInvalidWithoutInventingTimestamp(t *testing.T) {
	for _, input := range []string{"", " ", "not-a-date", "2026-02-30", "2026年13月1日", "2月30日", "-1 days ago", "1.5 hours ago", "10 weeks ago garbage", `"10天前`, `"); throw new Error("injected"); //`} {
		got, err := ConvertStrToUnix(input)
		if err == nil || !got.IsZero() {
			t.Fatalf("%q accepted as %s: %v", input, got, err)
		}
		if strings.Contains(err.Error(), "Error: injected") {
			t.Fatalf("input executed: %v", err)
		}
	}
	for _, input := range []string{"", "2026-10-29", "-1 days ago", "1.5 hours ago", `"10天前`} {
		got, err := ConvertAgoToUnix(input)
		if err == nil || !got.IsZero() {
			t.Fatalf("relative %q accepted as %s: %v", input, got, err)
		}
	}
}

func TestDateContractRelativeUnitsAndLeapDays(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		input    string
		expected time.Time
	}{
		{"0秒前", now}, {"5 seconds ago", now.Add(-5 * time.Second)}, {"2 mins ago", now.Add(-2 * time.Minute)},
		{"3小时前", now.Add(-3 * time.Hour)}, {"1 week ago", now.AddDate(0, 0, -7)}, {"2个月前", subtractCalendarMonths(now, 2)},
		{"1年前", subtractCalendarMonths(now, 12)},
	} {
		got, err := ConvertStrToUnix(tc.input)
		if err != nil || got.Sub(tc.expected) < -2*time.Second || got.Sub(tc.expected) > 2*time.Second {
			t.Fatalf("%q: got %s expected %s, %v", tc.input, got, tc.expected, err)
		}
	}
	for _, input := range []string{"2024年2月29日", "2024-02-29"} {
		got, err := ConvertStrToUnix(input)
		if err != nil || got.Year() != 2024 || got.Month() != time.February || got.Day() != 29 {
			t.Fatalf("%q: %s %v", input, got, err)
		}
	}
	got, err := ConvertStrToUnix("2月29日")
	year := time.Now().Year()
	leap := year%4 == 0 && (year%100 != 0 || year%400 == 0)
	if leap {
		if err != nil || got.Year() != year || got.Month() != time.February || got.Day() != 29 {
			t.Fatalf("current-year leap day: %s %v", got, err)
		}
	} else if err == nil {
		t.Fatalf("non-leap-year date accepted: %s", got)
	}
}

func TestDateResultRejectsInvalidAndHonorsCancellation(t *testing.T) {
	for _, input := range []string{"NaN", "", "1.5", "9223372036854775808"} {
		if got, err := parseUnixDateResult(input); err == nil || !got.IsZero() {
			t.Fatalf("%q: %s %v", input, got, err)
		}
	}
	for _, input := range []string{"0", "-1", "-315619200"} {
		if _, err := parseUnixDateResult(input); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ConvertStrToUnixContext(ctx, "2026-10-29"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}

// Match calendar subtraction at month ends without assuming every month has
// today's day number. Moment clamps these values instead of rolling forward.
func subtractCalendarMonths(now time.Time, months int) time.Time {
	target := time.Date(now.Year(), now.Month()-time.Month(months), 1, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), now.Location())
	last := time.Date(target.Year(), target.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
	return time.Date(target.Year(), target.Month(), min(now.Day(), last), now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), now.Location())
}
