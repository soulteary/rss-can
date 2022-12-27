package cacher_test

import (
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/cacher"
)

func TestRedis(t *testing.T) {
	instance := cacher.InitializeRedis(true)
	TestKey := "key" + time.Now().String()
	TestValue := "value"

	ret, err := cacher.GetDataFromRedis(instance, TestKey)
	if err != nil {
		t.Fatal("GetDataFromRedis failed", err)
	}
	if ret != "" {
		t.Fatal("GetDataFromRedis failed")
	}

	err = cacher.UpdateDataToRedis(instance, TestKey, TestValue)
	if err != nil {
		t.Fatal("UpdateDataToRedis failed")
	}
	ret, err = cacher.GetDataFromRedis(instance, TestKey)
	if err != nil {
		t.Fatal("GetDataFromRedis failed", err)
	}
	if ret != TestValue {
		t.Fatal("GetDataFromRedis failed")
	}

	err = cacher.DelDataByKeyFromRedis(instance, TestKey)
	if err != nil {
		t.Fatal("DelDataByKeyFromRedis failed")
	}
	ret, err = cacher.GetDataFromRedis(instance, TestKey)
	if err != nil {
		t.Fatal("GetDataFromRedis failed", err)
	}
	if ret != "" {
		t.Fatal("GetDataFromRedis failed")
	}

	err = cacher.UpdateDataToRedis(instance, TestKey, TestValue)
	if err != nil {
		t.Fatal("UpdateDataToRedis failed")
	}
	// redis minimal supported value is 1s
	err = cacher.SetDataExpireByKeyFromRedis(instance, TestKey, (time.Second * 1))
	if err != nil {
		t.Fatal("SetDataExpireByKeyFromRedis failed")
	}
	time.Sleep(time.Second*1 + time.Millisecond*10)
	ret, err = cacher.GetDataFromRedis(instance, TestKey)
	if err != nil {
		t.Fatal("GetDataFromRedis failed", err)
	}
	if ret != "" {
		t.Fatal("GetDataFromRedis failed")
	}

	err = cacher.Disconnect(instance)
	if err != nil {
		t.Fatal("Disconnect failed", err)
	}

	err = cacher.Disconnect(instance)
	if err == nil {
		t.Fatal("Disconnect failed")
	}
}

func TestInitializeRedis(t *testing.T) {
	ret := cacher.InitializeRedis(false)
	if ret != nil {
		t.Fatal("InitializeRedis failed")
	}
}
