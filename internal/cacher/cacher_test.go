package cacher_test

import (
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
)

func TestCacher(t *testing.T) {
	TestKey := "key" + time.Now().String()
	TestValue := "value"

	define.IN_MEMORY_CACHE = false
	define.REDIS = false

	if cacher.IsEnable() != false {
		t.Fatal("cacher.IsEnable failed")
	}

	ret, err := cacher.Get(TestKey)
	if err == nil {
		t.Fatal("cacher.Get failed")
	}
	if ret != "" {
		t.Fatal("cacher.Get failed")
	}

	err = cacher.Set(TestKey, TestValue)
	if err == nil {
		t.Fatal("cacher.Set failed")
	}

	err = cacher.Del(TestKey)
	if err == nil {
		t.Fatal("cacher.Del failed")
	}

	err = cacher.Expire(TestKey, time.Second*1)
	if err == nil {
		t.Fatal("cacher.Expire failed")
	}

	define.IN_MEMORY_CACHE = false
	define.REDIS = true

	if cacher.IsEnable() != true {
		t.Fatal("cacher.IsEnable failed")
	}

	ret, err = cacher.Get(TestKey)
	if err != nil {
		t.Fatal("cacher.Get failed")
	}
	if ret != "" {
		t.Fatal("cacher.Get failed")
	}

	err = cacher.Set(TestKey, TestValue)
	if err != nil {
		t.Fatal("cacher.Set failed")
	}
	ret, err = cacher.Get(TestKey)
	if err != nil {
		t.Fatal("cacher.Get failed")
	}
	if ret != TestValue {
		t.Fatal("cacher.Get failed")
	}

	err = cacher.Del(TestKey)
	if err != nil {
		t.Fatal("cacher.Del failed")
	}
	ret, err = cacher.Get(TestKey)
	if err != nil {
		t.Fatal("cacher.Get failed")
	}
	if ret != "" {
		t.Fatal("cacher.Get failed")
	}

	err = cacher.Set(TestKey, TestValue)
	if err != nil {
		t.Fatal("cacher.Set failed")
	}
	err = cacher.Expire(TestKey, (time.Second * 1))
	if err != nil {
		t.Fatal("cacher.Expire failed")
	}
	time.Sleep(time.Second * 2)
	ret, err = cacher.Get(TestKey)
	if err != nil {
		t.Fatal("cacher.Get failed")
	}
	if ret != "" {
		t.Fatal("cacher.Get failed")
	}

	define.IN_MEMORY_CACHE = true
	define.REDIS = false

	if cacher.IsEnable() != true {
		t.Fatal("cacher.IsEnable failed")
	}

	ret, err = cacher.Get(TestKey)
	if err != nil {
		t.Fatal("cacher.Get failed")
	}
	if ret != "" {
		t.Fatal("cacher.Get failed")
	}

	err = cacher.Set(TestKey, TestValue)
	if err != nil {
		t.Fatal("cacher.Set failed")
	}
	ret, err = cacher.Get(TestKey)
	if err != nil {
		t.Fatal("cacher.Get failed")
	}
	if ret != TestValue {
		t.Fatal("cacher.Get failed")
	}

	err = cacher.Del(TestKey)
	if err != nil {
		t.Fatal("cacher.Del failed")
	}
	ret, err = cacher.Get(TestKey)
	if err != nil {
		t.Fatal("cacher.Get failed")
	}
	if ret != "" {
		t.Fatal("cacher.Get failed")
	}

	err = cacher.Set(TestKey, TestValue)
	if err != nil {
		t.Fatal("cacher.Set failed")
	}
	err = cacher.Expire(TestKey, (time.Millisecond * 100))
	if err != nil {
		t.Fatal("cacher.Expire failed")
	}
	time.Sleep(time.Millisecond * 200)
	ret, err = cacher.Get(TestKey)
	if err != nil {
		t.Fatal("cacher.Get failed")
	}
	if ret != "" {
		t.Fatal("cacher.Get failed")
	}

}
