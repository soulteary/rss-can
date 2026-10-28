package jssdk

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/RSS-Can/internal/limits"
)

func TestProtocolReaderRejectsLongUnterminatedResponse(t *testing.T) {
	for _, data := range []string{strings.Repeat("x", limits.MaxJSMessageBytes+1), strings.Repeat("x", limits.MaxJSMessageBytes) + "\n"} {
		if line, err := readProtocolLine(bufio.NewReaderSize(strings.NewReader(data), 128)); err == nil || line != nil {
			t.Fatal("oversized protocol line accepted")
		}
	}
	got, err := readProtocolLine(bufio.NewReaderSize(strings.NewReader("{\"ready\":true}\n"), 2))
	if err != nil || !bytes.Equal(got, []byte("{\"ready\":true}\n")) {
		t.Fatalf("%q %v", got, err)
	}
}
func TestDenoRequestLimitAndWorkerRecovery(t *testing.T) {
	worker := &denoWorker{}
	defer worker.close()
	if value, err := worker.runContext(t.Context(), "", strings.Repeat("x", limits.MaxJSMessageBytes+1), "large.js", time.Second); err == nil || value != "" {
		t.Fatal("oversized JS request accepted")
	}
	if value, err := worker.runContext(t.Context(), "", strings.Repeat("\n", limits.MaxJSMessageBytes/2+1), "escaped-large.js", time.Second); err == nil || value != "" {
		t.Fatal("encoded oversized JS request accepted")
	}
	if value, err := worker.runContext(t.Context(), "", fmt.Sprintf("'x'.repeat(%d)", limits.MaxJSMessageBytes+1), "huge-result.js", time.Second); err == nil || value != "" {
		t.Fatal("oversized JS result accepted")
	}
	if value, err := worker.runContext(t.Context(), "", "'healthy'", "small.js", time.Second); err != nil || value != "healthy" {
		t.Fatalf("recovery %q %v", value, err)
	}
}
