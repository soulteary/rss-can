package fn_test

import (
	"testing"

	"github.com/soulteary/RSS-Can/internal/fn"
)

func TestJSONStringify(t *testing.T) {
	ret := fn.JSONStringify("true")

	if ret != `"true"` {
		t.Fatal("JSONStringify failed")
	}

	type Human struct {
		Name string `json:"name"`
		Age  int    `json:"age,omitempty"`
	}

	var people Human
	people.Name = "A"
	people.Age = 10

	ret = fn.JSONStringify(&people)
	if ret != `{"name":"A","age":10}` {
		t.Fatal("JSONStringify failed")
	}

	mockErr := map[string]interface{}{
		"foo": make(chan int),
	}
	ret = fn.JSONStringify(mockErr)
	if ret != "" {
		t.Fatal("JSONStringify failed")
	}
}
