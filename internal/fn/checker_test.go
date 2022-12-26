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
