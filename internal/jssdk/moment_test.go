package jssdk_test

import (
	"fmt"
	"testing"

	"github.com/soulteary/RSS-Can/internal/jssdk"
)

func TestConvertAgoToUnix(t *testing.T) {
	_, err := jssdk.ConvertAgoToUnix("10天前")
	if err != nil {
		t.Fatal("ConvertAgoToUnix failed: ", err)
	}

	_, err = jssdk.ConvertAgoToUnix(`"10天前`)
	if err == nil {
		t.Fatal("ConvertAgoToUnix failed")
	}
}

func TestConvertStrToUnix(t *testing.T) {
	timeUnix, err := jssdk.ConvertStrToUnix("10天前")
	if err != nil {
		t.Fatal("ConvertStrToUnix failed: ", err)
	}
	fmt.Println(timeUnix)

	_, err = jssdk.ConvertStrToUnix(`"10天前`)
	if err == nil {
		t.Fatal("ConvertStrToUnix failed")
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
