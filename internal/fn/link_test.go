package fn_test

import (
	"fmt"
	"testing"

	"github.com/soulteary/RSS-Can/internal/fn"
)

func TestLinkResolveRelative(t *testing.T) {
	const target = "http://example.com/"
	ret, err := fn.LinkResolveRelative("../", "http://example.com/abc")
	if err != nil {
		t.Fatal("LinkResolveRelative failed: ", err)
	}
	if ret != target {
		t.Fatal("LinkResolveRelative failed")
	}

	ret, err = fn.LinkResolveRelative("../../", "http://example.com/abc/def")
	if err != nil {
		t.Fatal("LinkResolveRelative failed: ", err)
	}
	if ret != target {
		t.Fatal("LinkResolveRelative failed")
	}

	ret, err = fn.LinkResolveRelative("../.././", "http://example.com/abc/def")
	if err != nil {
		t.Fatal("LinkResolveRelative failed: ", err)
	}
	if ret != target {
		t.Fatal("LinkResolveRelative failed")
	}

	ret, err = fn.LinkResolveRelative(".././././.././", "http://example.com/abc/def")
	if err != nil {
		t.Fatal("LinkResolveRelative failed: ", err)
	}
	if ret != target {
		t.Fatal("LinkResolveRelative failed")
	}

	ret, err = fn.LinkResolveRelative("../../../../", "http://example.com/abc/def")
	if err != nil {
		t.Fatal("LinkResolveRelative failed: ", err)
	}
	if ret != target {
		t.Fatal("LinkResolveRelative failed")
	}

	ret, err = fn.LinkResolveRelative("../../../../", "http://example.com/abc/def.html")
	if err != nil {
		t.Fatal("LinkResolveRelative failed: ", err)
	}
	if ret != target {
		t.Fatal("LinkResolveRelative failed")
	}

	ret, err = fn.LinkResolveRelative("http://example.com/abc/abcd.html", "http://example.com/abc/def.html")
	if err != nil {
		t.Fatal("LinkResolveRelative failed: ", err)
	}
	if ret != "http://example.com/abc/abcd.html" {
		t.Fatal("LinkResolveRelative failed")
	}

	ret, err = fn.LinkResolveRelative("http://example.com/abc/abcd/efg.html", "http://example.com/abc/def.html")
	if err != nil {
		t.Fatal("LinkResolveRelative failed: ", err)
	}
	if ret != "http://example.com/abc/abcd/efg.html" {
		t.Fatal("LinkResolveRelative failed")
	}

	_, err = fn.LinkResolveRelative("../../../../", "htt!p://example.com/abc/def.html")
	if err == nil {
		t.Fatal("LinkResolveRelative failed")
	}

	_, err = fn.LinkResolveRelative("htt!p://example.com/abc/def.html", "http://example.com/abc/def.html")
	if err == nil {
		t.Fatal("LinkResolveRelative failed")
	}

	ret, err = fn.LinkResolveRelative(target, "http://example.com/abc/def.html")
	if err != nil {
		t.Fatal("LinkResolveRelative failed")
	}
	if ret != target {
		t.Fatal("LinkResolveRelative failed")
	}

	ret, err = fn.LinkResolveRelative(target+"####htt####p://", "http://example.com/abc/def.html")
	if err != nil {
		t.Fatal("LinkResolveRelative failed")
	}

	fmt.Println(ret)
}
