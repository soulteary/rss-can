package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/feedservice"
	"github.com/soulteary/RSS-Can/internal/logger"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/RSS-Can/internal/source"
)

type sourceRequest struct {
	Spec     source.Spec `json:"spec"`
	Revision uint64      `json:"revision,omitempty"`
}

func (a *AccessController) preview(c fiber.Ctx, spec source.Spec) ([]define.InfoItem, error) {
	spec = spec.Normalized()
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	if err := network.PublicPolicy().ValidateURL(spec.URL); err != nil {
		return nil, err
	}
	if a.Preview == nil {
		return nil, errors.New("source preview unavailable")
	}
	a.previewOnce.Do(func() { a.previewSlots = make(chan struct{}, 2) })
	select {
	case a.previewSlots <- struct{}{}:
		defer func() { <-a.previewSlots }()
	default:
		return nil, errPreviewBusy
	}
	items, err := a.Preview(c.Context(), spec)
	if err != nil {
		return nil, err
	}
	if err := source.ValidateItems(items); err != nil {
		return nil, err
	}
	return items[:min(5, len(items))], nil
}

var errPreviewBusy = errors.New("preview capacity reached")

func sourceFailure(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, access.ErrDenied):
		return c.Status(404).JSON(fiber.Map{"code": "SOURCE_NOT_FOUND", "msg": "source not found"})
	case errors.Is(err, access.ErrConflict):
		return c.Status(409).JSON(fiber.Map{"code": "SOURCE_CHANGED", "msg": "source changed; reload before saving"})
	case errors.Is(err, access.ErrSourceLimit):
		c.Set("Retry-After", "60")
		return c.Status(429).JSON(fiber.Map{"code": "SOURCE_LIMIT", "msg": "source limit reached; delete an unused source first"})
	case errors.Is(err, network.ErrFetchPolicy):
		return c.Status(400).JSON(fiber.Map{"code": "SOURCE_BLOCKED", "msg": "only public HTTP(S) sources on web ports are allowed"})
	case errors.Is(err, errPreviewBusy):
		c.Set("Retry-After", "5")
		return c.Status(429).JSON(fiber.Map{"code": "RATE_LIMITED", "msg": "preview is busy; retry shortly"})
	case errors.Is(err, source.ErrNoMatch):
		return c.Status(422).JSON(fiber.Map{"code": "RULE_NO_MATCH", "msg": "no items matched; check the list, title and link selectors"})
	case errors.Is(err, source.ErrSelector):
		return c.Status(400).JSON(fiber.Map{"code": "SIMPLE_SELECTOR_REQUIRED", "msg": "use simple tag, class or ID selectors without combinators"})
	case errors.Is(err, source.ErrDocumentBudget):
		return c.Status(422).JSON(fiber.Map{"code": "SOURCE_COMPLEXITY", "msg": "page exceeds the managed source parsing budget"})
	default:
		return c.Status(422).JSON(fiber.Map{"code": "SOURCE_INVALID", "msg": "preview failed; check the URL, CSS selectors and source availability"})
	}
}

func (a *AccessController) previewManaged() fiber.Handler {
	return func(c fiber.Ctx) error {
		if !a.writeOrigin(c) {
			return c.SendStatus(403)
		}
		var body sourceRequest
		if decodeAccessBody(c, &body) != nil {
			return c.SendStatus(400)
		}
		items, err := a.preview(c, body.Spec)
		if err != nil {
			return sourceFailure(c, err)
		}
		return c.JSON(fiber.Map{"items": items, "mode": "ssr"})
	}
}

func (a *AccessController) saveManaged(update bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		if !a.writeOrigin(c) {
			return c.SendStatus(403)
		}
		var body sourceRequest
		if decodeAccessBody(c, &body) != nil {
			return c.SendStatus(400)
		}
		subject, _ := c.Locals(subjectLocal).(string)
		id := c.Params("id")
		if update {
			current, ok := a.Store.Managed(id)
			if !ok || current.Owner != subject {
				return sourceFailure(c, access.ErrDenied)
			}
			if body.Revision != current.Revision {
				return sourceFailure(c, access.ErrConflict)
			}
		}
		if _, err := a.preview(c, body.Spec); err != nil {
			return sourceFailure(c, err)
		}
		if err := c.Context().Err(); err != nil {
			return sourceFailure(c, err)
		}
		var src access.Source
		var err error
		if update {
			src, err = a.Store.UpdateManaged(subject, id, body.Spec, body.Revision)
		} else {
			src, err = a.Store.CreateManaged(subject, body.Spec)
		}
		if err != nil {
			return sourceFailure(c, err)
		}
		if logger.Instance != nil {
			logger.Instance.Infow("declarative source saved", "subject", subject, "source", src.ID, "revision", src.Revision)
		}
		status := http.StatusCreated
		if update {
			status = http.StatusOK
		}
		return c.Status(status).JSON(fiber.Map{"source": src})
	}
}

func (a *AccessController) rollbackManaged() fiber.Handler {
	return func(c fiber.Ctx) error {
		if !a.writeOrigin(c) {
			return c.SendStatus(403)
		}
		var body struct {
			Revision uint64 `json:"revision"`
		}
		if decodeAccessBody(c, &body) != nil {
			return c.SendStatus(400)
		}
		subject, _ := c.Locals(subjectLocal).(string)
		src, err := a.Store.RollbackManaged(subject, c.Params("id"), body.Revision)
		if err != nil {
			return sourceFailure(c, err)
		}
		if logger.Instance != nil {
			logger.Instance.Infow("declarative source rolled back", "subject", subject, "source", src.ID, "revision", src.Revision)
		}
		return c.JSON(fiber.Map{"source": src})
	}
}

func sourceWizard() fiber.Handler {
	body := GetPageByName("sources.html")
	return func(c fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		c.Set("Cache-Control", "private, no-store")
		c.Set("Referrer-Policy", "no-referrer")
		c.Set("Content-Security-Policy", strings.Join([]string{"default-src 'none'", "script-src 'self'", "style-src 'self'", "connect-src 'self'", "base-uri 'none'", "form-action 'self'", "frame-ancestors 'none'"}, "; "))
		return c.Send(body)
	}
}

func (a *AccessController) listGrants() fiber.Handler {
	return func(c fiber.Ctx) error {
		subject, _ := c.Locals(subjectLocal).(string)
		return c.JSON(fiber.Map{"grants": a.Store.Grants(subject)})
	}
}

func (a *AccessController) deleteManaged() fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Revision uint64 `json:"revision"`
		}
		if decodeAccessBody(c, &body) != nil {
			return c.SendStatus(400)
		}
		subject, _ := c.Locals(subjectLocal).(string)
		if err := a.Store.DeleteManaged(subject, c.Params("id"), body.Revision); err != nil {
			return sourceFailure(c, err)
		}
		if logger.Instance != nil {
			logger.Instance.Infow("declarative source deleted", "subject", subject, "source", c.Params("id"))
		}
		return c.SendStatus(204)
	}
}

type refreshableFeeds interface {
	Refresh(context.Context, string, string) (feedservice.Representation, error)
}

func (a *AccessController) refreshManaged(feeds FeedGenerator) fiber.Handler {
	return func(c fiber.Ctx) error {
		subject, _ := c.Locals(subjectLocal).(string)
		id := c.Params("id")
		src, ok := a.Store.Source(id)
		if !ok || src.Owner != subject {
			return sourceFailure(c, access.ErrDenied)
		}
		version, allowed := a.Store.CanRead(subject, id)
		if !allowed {
			return sourceFailure(c, access.ErrDenied)
		}
		ctx := access.WithScope(c.Context(), subject+"\x00"+id, version)
		var representation feedservice.Representation
		var err error
		if snapshots, ok := feeds.(refreshableFeeds); ok {
			representation, err = snapshots.Refresh(ctx, id, "rss")
		} else {
			_, err = feeds.Generate(ctx, id, "rss")
			representation.FetchedAt = time.Now()
		}
		if err != nil {
			c.Set("Retry-After", "5")
			return c.Status(503).JSON(fiber.Map{"code": "REFRESH_FAILED", "msg": "refresh failed; an existing successful snapshot is retained when available"})
		}
		if representation.FetchedAt.IsZero() {
			representation.FetchedAt = time.Now()
		}
		return c.JSON(fiber.Map{"fetched_at": representation.FetchedAt, "changed_at": representation.ChangedAt, "stale": representation.Stale})
	}
}
