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
	time.Sleep(time.Second * 2)
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

func TestRedisLegacyJSONBodiesRemainUnchanged(t *testing.T) {
	instance := cacher.InitializeRedis(true)
	defer cacher.Disconnect(instance)

	for _, body := range []string{`"hello"`, `null`, `{"title":"example"}`} {
		key := t.Name() + body
		if err := instance.Set(t.Context(), key, body, time.Minute).Err(); err != nil {
			t.Fatal(err)
		}

		got, err := cacher.GetDataFromRedis(instance, key)
		if err != nil {
			t.Fatal(err)
		}
		if got != body {
			t.Fatalf("legacy body changed: got %q, want %q", got, body)
		}
		if err := cacher.DelDataByKeyFromRedis(instance, key); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInitializeRedis(t *testing.T) {
	ret := cacher.InitializeRedis(false)
	if ret != nil {
		t.Fatal("InitializeRedis failed")
	}
}
