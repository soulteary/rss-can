package define_test

import (
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/fn"
)

func TestMixupRemoteBodySanitized(t *testing.T) {
	code := define.ERROR_CODE_NULL
	status := "dummy status"
	date := time.Now()
	body := "hello world"

	object := define.MixupRemoteBodySanitized(code, status, date, body)

	if !(object.Code == code && object.Status == status && object.Date == date && object.Body == body) {
		t.Fatal("MixupRemoteBodySanitized failed")
	}
}

func TestMixupBodyParsed(t *testing.T) {
	code := define.ERROR_CODE_NULL
	status := "dummy status"
	date := time.Now()
	var item []define.InfoItem

	object := define.MixupBodyParsed(code, status, date, item)
	if !(object.Code == code && object.Status == status && object.Date == date) {
		t.Fatal("MixupBodyParsed failed")
	}

	f1 := fn.JSONStringify(object.Body)
	f2 := fn.JSONStringify(item)
	if f1 != f2 {
		t.Fatal("MixupBodyParsed failed")
	}
}
