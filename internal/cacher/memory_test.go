package cacher_test

import (
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/cacher"
	"github.com/soulteary/RSS-Can/internal/define"
)

func TestMemory(t *testing.T) {
	instance := cacher.InitializeMemory(true, define.DEFAULT_IN_MEMORY_CACHE_STORE_NAME)

	ret, err := cacher.GetDataFromMemory(instance, "key")
	if err != nil {
		t.Fatal("GetDataFromMemory failed", err)
	}
	if string(ret) != "" {
		t.Fatal("GetDataFromMemory failed")
	}

	cacher.UpdateDataToMemory(instance, "key", "value")
	ret, err = cacher.GetDataFromMemory(instance, "key")
	if err != nil {
		t.Fatal("GetDataFromMemory failed", err)
	}
	if string(ret) != "value" {
		t.Fatal("GetDataFromMemory failed")
	}

	cacher.DelDataByKeyFromMemory(instance, "key")
	ret, err = cacher.GetDataFromMemory(instance, "key")
	if err != nil {
		t.Fatal("GetDataFromMemory failed", err)
	}
	if string(ret) != "" {
		t.Fatal("GetDataFromMemory failed")
	}

	cacher.UpdateDataToMemory(instance, "key", "value")
	cacher.SetDataExpireByKeyFromMemory(instance, "key", time.Millisecond*100)
	time.Sleep(time.Millisecond * 200)
	ret, err = cacher.GetDataFromMemory(instance, "key")
	if err != nil {
		t.Fatal("GetDataFromMemory failed", err)
	}
	if string(ret) != "" {
		t.Fatal("GetDataFromMemory failed")
	}

	cacher.UpdateDataToMemory(instance, "key", "value")
	cacher.FlushDataFromMemory(instance)
	ret, err = cacher.GetDataFromMemory(instance, "key")
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

	cacher.UpdateDataToMemory(instance, "key", "value")
	empty = cacher.IsMemoryEmpty(instance)
	if empty {
		t.Fatal("IsMemoryEmpty failed")
	}
}
