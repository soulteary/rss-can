package jssdk_test

import (
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/jssdk"
)

func TestConvertAgoToUnix(t *testing.T) {
	for _, input := range []string{"10天前", "10 days ago", "10 天前", "10天 前"} {
		before := time.Now().AddDate(0, 0, -10)
		got, err := jssdk.ConvertAgoToUnix(input)
		if err != nil || got.Sub(before) < -time.Minute || got.Sub(before) > time.Minute {
			t.Fatalf("%q: %s %v", input, got, err)
		}
	}
}

func TestConvertStrToUnix(t *testing.T) {
	for _, tc := range []struct {
		input string
		year  int
		month time.Month
		day   int
	}{
		{"2022年", 2022, time.January, 1}, {"2022年1月", 2022, time.January, 1}, {"2022年12月12日", 2022, time.December, 12},
		{"12月12日", time.Now().Year(), time.December, 12}, {"2022-12-12", 2022, time.December, 12}, {"1960-01-01", 1960, time.January, 1},
	} {
		got, err := jssdk.ConvertStrToUnix(tc.input)
		if err != nil || got.Year() != tc.year || got.Month() != tc.month || got.Day() != tc.day {
			t.Fatalf("%q: got %s (%v)", tc.input, got, err)
		}
	}
}

func TestMomentUpgradePreservesDateValues(t *testing.T) {
	for _, input := range []string{"2022-12-12", "2022年12月12日"} {
		got, err := jssdk.ConvertStrToUnix(input)
		if err != nil || got.Year() != 2022 || got.Month() != time.December || got.Day() != 12 {
			t.Fatalf("%q: %s %v", input, got, err)
		}
	}
}
