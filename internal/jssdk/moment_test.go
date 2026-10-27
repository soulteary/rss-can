package jssdk_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/jssdk"
)

func TestConvertAgoToUnix(t *testing.T) {
	_, err := jssdk.ConvertAgoToUnix("10天前")
	if err != nil {
		t.Fatal("ConvertAgoToUnix failed: ", err)
	}

	_, err = jssdk.ConvertAgoToUnix(`"10天前`)
	// Quotes are input data, not JavaScript delimiters.
	if err != nil {
		t.Fatal("quoted date must not become executable source:", err)
	}
}

func TestConvertStrToUnix(t *testing.T) {
	timeUnix, err := jssdk.ConvertStrToUnix("10天前")
	if err != nil {
		t.Fatal("ConvertStrToUnix failed: ", err)
	}
	fmt.Println(timeUnix)

	_, err = jssdk.ConvertStrToUnix(`"10天前`)
	// Quotes are input data, not JavaScript delimiters.
	if err != nil {
		t.Fatal("quoted date must not become executable source:", err)
	}

	timeUnix, err = jssdk.ConvertStrToUnix("2022年")
	if err != nil {
		t.Fatal("ConvertStrToUnix failed: ", err)
	}
	fmt.Println(timeUnix)

	timeUnix, err = jssdk.ConvertStrToUnix("2022年1月")
	if err != nil {
		t.Fatal("ConvertStrToUnix failed: ", err)
	}
	fmt.Println(timeUnix)

	timeUnix, err = jssdk.ConvertStrToUnix("2022年12月12日")
	if err != nil {
		t.Fatal("ConvertStrToUnix failed: ", err)
	}
	fmt.Println(timeUnix)

	timeUnix, err = jssdk.ConvertStrToUnix("12月12日")
	if err != nil {
		t.Fatal("ConvertStrToUnix failed: ", err)
	}
	fmt.Println(timeUnix)

}

func TestMomentUpgradePreservesDateValues(t *testing.T) {
	for _, input := range []string{"2022-12-12", "2022年12月12日"} {
		got, err := jssdk.ConvertStrToUnix(input)
		if err != nil || got.Year() != 2022 || got.Month() != time.December || got.Day() != 12 {
			t.Errorf("%q: got %s, %v", input, got, err)
		}
	}
	before := time.Now().AddDate(0, 0, -10)
	got, err := jssdk.ConvertAgoToUnix("10天前")
	if err != nil || got.Sub(before) < -time.Minute || got.Sub(before) > time.Minute {
		t.Fatalf("relative date changed: %s, %v", got, err)
	}
}
