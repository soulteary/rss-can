package fn_test

import (
	"testing"

	"github.com/soulteary/RSS-Can/internal/fn"
)

func TestIsDomTagName(t *testing.T) {
	for _, tag := range fn.DomList {
		ret := fn.IsDomTagName(tag)
		if ret != true {
			t.Fatal("An error occurred while checking whether the element is a DOM Tag")
		}
	}

	ret := fn.IsDomTagName("hhhh")
	if ret == true {
		t.Fatal("An error occurred while checking whether the element is a DOM Tag")
	}
}

func TestIsCssSelector(t *testing.T) {
	ret := fn.IsCssSelector(".a")
	if !ret {
		t.Fatal("Error checking if element is a CSS Selector")
	}

	ret = fn.IsCssSelector("h1 a")
	if !ret {
		t.Fatal("Error checking if element is a CSS Selector")
	}

	ret = fn.IsCssSelector("#id")
	if !ret {
		t.Fatal("Error checking if element is a CSS Selector")
	}

	ret = fn.IsCssSelector("> nth-child(1)")
	if !ret {
		t.Fatal("Error checking if element is a CSS Selector")
	}
}

func TestIsStrInArray(t *testing.T) {
	ret := fn.IsStrInArray(fn.DomList, "a")
	if !ret {
		t.Fatal("Checking string array contains data failed")
	}

	ret = fn.IsStrInArray(fn.DomList, "a!!")
	if ret {
		t.Fatal("Checking string array contains data failed")
	}
}

func TestIsVaildPortRange(t *testing.T) {
	ret := fn.IsVaildPortRange(0)
	if ret {
		t.Fatal("IsVaildPortRange failed")
	}

	ret = fn.IsVaildPortRange(-1)
	if ret {
		t.Fatal("IsVaildPortRange failed")
	}

	ret = fn.IsVaildPortRange(1000000)
	if ret {
		t.Fatal("IsVaildPortRange failed")
	}

	ret = fn.IsVaildPortRange(3000)
	if !ret {
		t.Fatal("IsVaildPortRange failed")
	}
}

func TestIsNotEmptyAndNotDefaultString(t *testing.T) {
	ret := fn.IsNotEmptyAndNotDefaultString("", "d")
	if ret {
		t.Fatal("IsNotEmptyAndNotDefaultString failed")
	}

	ret = fn.IsNotEmptyAndNotDefaultString("d", "d")
	if ret {
		t.Fatal("IsNotEmptyAndNotDefaultString failed")
	}
}

func TestIsVaildLogLevel(t *testing.T) {
	ret := fn.IsVaildLogLevel("info")
	if !ret {
		t.Fatal("IsVaildLogLevel failed")
	}
	ret = fn.IsVaildLogLevel("error")
	if !ret {
		t.Fatal("IsVaildLogLevel failed")
	}
	ret = fn.IsVaildLogLevel("warn")
	if !ret {
		t.Fatal("IsVaildLogLevel failed")
	}
	ret = fn.IsVaildLogLevel("debug")
	if !ret {
		t.Fatal("IsVaildLogLevel failed")
	}
	ret = fn.IsVaildLogLevel("not-vaild")
	if ret {
		t.Fatal("IsVaildLogLevel failed")
	}
}

func TestIsBoolString(t *testing.T) {
	ret := fn.IsBoolString("true")
	if !ret {
		t.Fatal("IsBoolString failed")
	}
	ret = fn.IsBoolString("on")
	if !ret {
		t.Fatal("IsBoolString failed")
	}
	ret = fn.IsBoolString("1")
	if !ret {
		t.Fatal("IsBoolString failed")
	}
	ret = fn.IsBoolString("ON")
	if !ret {
		t.Fatal("IsBoolString failed")
	}

	ret = fn.IsBoolString("not-vaild")
	if ret {
		t.Fatal("IsBoolString failed")
	}
}
