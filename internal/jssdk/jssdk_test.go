package jssdk_test

import (
	"strings"
	"testing"

	"github.com/soulteary/RSS-Can/internal/jssdk"
)

func TestGenerateGetConfigWithRule(t *testing.T) {
	const found = `console.log("hello world")`
	ret := jssdk.GenerateGetConfigWithRule([]byte(found))
	if !strings.Contains(ret, found) {
		t.Fatal("GenerateGetConfigWithRule failed")
	}
}

func TestGenerateCSRInjectParser(t *testing.T) {
	const found = `console.log("hello world")`
	ret := jssdk.GenerateCSRInjectParser([]byte(found))
	if !strings.Contains(ret, found) {
		t.Fatal("GenerateCSRInjectParser failed")
	}
}
