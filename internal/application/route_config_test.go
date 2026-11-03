package application

import (
	"context"
	"errors"
	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/server"
	"strings"
	"testing"
)

func TestNewRejectsReservedFeedPathBeforeOpeningResources(t *testing.T) {
	_, err := New(context.Background(), Config{Server: server.Options{FeedPath: "/sources", Access: &access.Config{}}})
	if err == nil || !strings.Contains(err.Error(), "RSS_HTTP_FEED_PATH") {
		t.Fatal(err)
	}
	hint := ConfigurationDiagnostic(errors.New("RSS_HTTP_FEED_PATH secret-value"))
	if hint.OK || strings.Contains(hint.Message, "secret-value") || !strings.Contains(hint.Message, "/feed") {
		t.Fatal(hint)
	}
}
