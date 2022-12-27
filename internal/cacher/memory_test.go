package cacher_test

import (
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
)

func TestMemory(t *testing.T) {
	instance := cacher.InitializeMemory(true, define.DEFAULT_IN_MEMORY_CACHE_STORE_NAME)
	TestKey := "key" + time.Now().String()
	TestValue := "value"

	ret, err := cacher.GetDataFromMemory(instance, TestKey)
	if err != nil {
		t.Fatal("GetDataFromMemory failed", err)
	}
	if string(ret) != "" {
		t.Fatal("GetDataFromMemory failed")
	}

	cacher.UpdateDataToMemory(instance, TestKey, TestValue)
	ret, err = cacher.GetDataFromMemory(instance, TestKey)
	if err != nil {
		t.Fatal("GetDataFromMemory failed", err)
	}
	if string(ret) != TestValue {
		t.Fatal("GetDataFromMemory failed")
	}

	cacher.DelDataByKeyFromMemory(instance, TestKey)
	ret, err = cacher.GetDataFromMemory(instance, TestKey)
	if err != nil {
		t.Fatal("GetDataFromMemory failed", err)
	}
	if string(ret) != "" {
		t.Fatal("GetDataFromMemory failed")
	}

	cacher.UpdateDataToMemory(instance, TestKey, TestValue)
	cacher.SetDataExpireByKeyFromMemory(instance, TestKey, time.Millisecond*50)
	time.Sleep(time.Millisecond * 80)
	ret, err = cacher.GetDataFromMemory(instance, TestKey)
	if err != nil {
		t.Fatal("GetDataFromMemory failed", err)
	}
	if string(ret) != "" {
		t.Fatal("GetDataFromMemory failed")
	}

	cacher.UpdateDataToMemory(instance, TestKey, TestValue)
	cacher.FlushDataFromMemory(instance)
	ret, err = cacher.GetDataFromMemory(instance, TestKey)
	if err != nil {
		t.Fatal("GetDataFromMemory failed", err)
	}
	if string(ret) != "" {
		t.Fatal("GetDataFromMemory failed")
	}

	empty := cacher.IsMemoryEmpty(instance)
	if !empty {
		t.Fatal("IsMemoryEmpty failed")
	}

	cacher.UpdateDataToMemory(instance, TestKey, TestValue)
	empty = cacher.IsMemoryEmpty(instance)
	if empty {
		t.Fatal("IsMemoryEmpty failed")
	}
}

func TestInitializeMemory(t *testing.T) {
	ret := cacher.InitializeMemory(false, "")
	if ret != nil {
		t.Fatal("InitializeMemory failed")
	}
}
