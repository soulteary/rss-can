package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/RSS-Can/internal/access"
	"github.com/soulteary/RSS-Can/internal/logger"
)

const subjectLocal = "rss.subject"
const sourceLocal = "rss.source"

type accessSubjectKey struct{}

// AccessController is injected by the application so its durable store has an
// explicit lifecycle. Role/scope headers are never trusted as authorization.
type AccessController struct {
	Config    access.Config
	Store     *access.Store
	Directory access.Checker
}

func (a *AccessController) subject(c fiber.Ctx) (string, error) {
	if a == nil || a.Store == nil || !a.Config.Trusted(c.RequestCtx().RemoteAddr().String()) {
		return "", access.ErrDenied
	}
	var subject string
	count := 0
	c.Request().Header.VisitAll(func(k, v []byte) {
		if strings.EqualFold(string(k), "X-Auth-User") {
			count++
			subject = string(v)
		}
	})
	if count != 1 || subject == "" || len(subject) > 256 || strings.TrimSpace(subject) != subject || strings.ContainsAny(subject, " ,/\\\r\n\t") {
		return "", access.ErrDenied
	}
	if err := a.active(c.Context(), subject); err != nil {
		return "", err
	}
	return subject, nil
}

func (a *AccessController) active(ctx context.Context, subject string) error {
	if !a.Store.Active(subject) {
		return access.ErrDenied
	}
	if a.Directory == nil {
		return errors.New("directory is unavailable")
	}
	return a.Directory.Active(ctx, subject)
}

func accessFailure(c fiber.Ctx, err error) error {
	c.Set("Cache-Control", "private, no-store")
	if errors.Is(err, access.ErrDenied) {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"code": "AUTH_REQUIRED", "msg": "authentication required"})
	}
	return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"code": "AUTH_UNAVAILABLE", "msg": "authorization service unavailable"})
}

func (a *AccessController) requireSubject() fiber.Handler {
	return func(c fiber.Ctx) error {
		subject, err := a.subject(c)
		if err != nil {
			return accessFailure(c, err)
		}
		c.Locals(subjectLocal, subject)
		c.SetContext(context.WithValue(c.Context(), accessSubjectKey{}, subject))
		c.Set("Cache-Control", "private, no-store")
		return c.Next()
	}
}

func (a *AccessController) requireFeed(machine bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Denials, including hidden-resource 404s, must not be retained by a
		// shared cache or turned into a conditional success response.
		c.Set("Cache-Control", "private, no-store")
		if a == nil || a.Store == nil {
			return accessFailure(c, errors.New("access is unavailable"))
		}
		format := strings.ToLower(c.Params("type"))
		source := c.Params("id")
		subject := ""
		var version uint64
		header := c.Get("Authorization")
		headerCount := 0
		c.Request().Header.VisitAll(func(k, v []byte) {
			if strings.EqualFold(string(k), "Authorization") {
				headerCount++
			}
		})
		if headerCount > 1 {
			return accessFailure(c, access.ErrDenied)
		}
		if machine || header != "" {
			id, secret := c.Params("selector"), c.Params("secret")
			if machine && header != "" {
				return accessFailure(c, access.ErrDenied)
			}
			if !machine {
				raw, ok := strings.CutPrefix(header, "Bearer ")
				if !ok {
					return accessFailure(c, access.ErrDenied)
				}
				id, secret, ok = strings.Cut(raw, ".")
				if !ok {
					return accessFailure(c, access.ErrDenied)
				}
			}
			grant, v, err := a.Store.ResolveGrant(id, secret, format, time.Now())
			if err != nil {
				return accessFailure(c, err)
			}
			if source != "" && source != grant.Source {
				return accessFailure(c, access.ErrDenied)
			}
			source, subject, version = grant.Source, grant.Subject, v
			if err = a.active(c.Context(), subject); err != nil {
				return accessFailure(c, err)
			}
			// Recheck after the directory round trip so a concurrent local revoke is
			// never hidden by a successful upstream status response.
			if _, version, err = a.Store.ResolveGrant(id, secret, format, time.Now()); err != nil {
				return accessFailure(c, err)
			}
		} else {
			v, public := a.Store.CanRead("", source)
			version = v
			if !public {
				var err error
				subject, err = a.subject(c)
				if err != nil {
					return accessFailure(c, err)
				}
				var allowed bool
				version, allowed = a.Store.CanRead(subject, source)
				if !allowed {
					return c.Status(http.StatusNotFound).JSON(fiber.Map{"code": "SOURCE_NOT_FOUND", "msg": "source not found"})
				}
			}
		}
		c.Locals(subjectLocal, subject)
		c.Locals(sourceLocal, source)
		c.Set("Cache-Control", "private, no-cache")
		c.Set("Referrer-Policy", "no-referrer")
		c.SetContext(access.WithScope(c.Context(), subject+"\x00"+source, version))
		return c.Next()
	}
}

func decodeAccessBody(c fiber.Ctx, out any) error {
	if len(c.Body()) > 64<<10 {
		return access.ErrInvalid
	}
	d := json.NewDecoder(strings.NewReader(string(c.Body())))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return access.ErrInvalid
	}
	return nil
}

func (a *AccessController) writeOrigin(c fiber.Ctx) bool {
	return c.Get("X-RSS-Admin-Action") == "1" && c.Get("Origin") == a.Config.PublicOrigin && (c.Get("Sec-Fetch-Site") == "" || c.Get("Sec-Fetch-Site") == "same-origin")
}

// accessMutation is intentionally independent of authentication: a non-simple
// header also protects credential-bearing operator clients from browser CSRF.
func accessMutation() fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Get("X-RSS-Admin-Action") != "1" {
			return c.SendStatus(http.StatusForbidden)
		}
		return c.Next()
	}
}

func (a *AccessController) listSources() fiber.Handler {
	return func(c fiber.Ctx) error {
		subject, _ := c.Locals(subjectLocal).(string)
		sources := a.Store.Sources(subject)
		sort.Slice(sources, func(i, j int) bool { return sources[i].ID < sources[j].ID })
		return c.JSON(fiber.Map{"sources": sources})
	}
}

func (a *AccessController) putSource(exists func(string) bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var src access.Source
		if decodeAccessBody(c, &src) != nil || !exists(src.ID) {
			return c.Status(400).JSON(fiber.Map{"msg": "invalid source or unknown rule"})
		}
		if err := a.Store.PutSource(src); err != nil {
			return c.Status(400).JSON(fiber.Map{"msg": "source update failed"})
		}
		if logger.Instance != nil {
			logger.Instance.Infow("source access updated", "source", src.ID, "authentication", "admin_token")
		}
		c.Set("Cache-Control", "no-store")
		return c.SendStatus(204)
	}
}

func (a *AccessController) setPrincipal() fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Subject  string `json:"subject"`
			Disabled bool   `json:"disabled"`
		}
		if decodeAccessBody(c, &body) != nil {
			return c.SendStatus(400)
		}
		if err := a.Store.SetPrincipal(body.Subject, body.Disabled); err != nil {
			return c.Status(400).JSON(fiber.Map{"msg": "principal update failed"})
		}
		if logger.Instance != nil {
			logger.Instance.Infow("principal access updated", "subject", body.Subject, "disabled", body.Disabled, "authentication", "admin_token")
		}
		return c.SendStatus(204)
	}
}

func (a *AccessController) createGrant() fiber.Handler {
	return func(c fiber.Ctx) error {
		if !a.writeOrigin(c) {
			return c.SendStatus(403)
		}
		var body struct {
			Formats []string `json:"formats"`
			Days    int      `json:"days"`
		}
		if decodeAccessBody(c, &body) != nil {
			return c.SendStatus(400)
		}
		if body.Days == 0 {
			body.Days = 90
		}
		if body.Days < 1 || body.Days > 365 {
			return c.SendStatus(400)
		}
		if len(body.Formats) == 0 {
			body.Formats = []string{"rss"}
		}
		subject, _ := c.Locals(subjectLocal).(string)
		g, secret, err := a.Store.CreateGrant(subject, c.Params("id"), body.Formats, time.Now().Add(time.Duration(body.Days)*24*time.Hour))
		if errors.Is(err, access.ErrDenied) {
			return c.SendStatus(404)
		}
		if errors.Is(err, access.ErrGrantLimit) {
			return c.Status(http.StatusTooManyRequests).JSON(fiber.Map{"code": "GRANT_LIMIT", "msg": "reader credential limit reached; revoke an unused credential"})
		}
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"msg": "grant creation failed"})
		}
		c.Set("Cache-Control", "no-store")
		if logger.Instance != nil {
			logger.Instance.Infow("reader grant created", "subject", subject, "source", g.Source, "grant_id", g.ID)
		}
		return c.Status(201).JSON(fiber.Map{"id": g.ID, "token": g.ID + "." + secret, "url": a.Config.PublicOrigin + "/s/" + g.ID + "/" + secret + "/" + g.Formats[0], "expires_at": g.ExpiresAt})
	}
}

func (a *AccessController) revokeGrant() fiber.Handler {
	return func(c fiber.Ctx) error {
		if !a.writeOrigin(c) {
			return c.SendStatus(403)
		}
		subject, _ := c.Locals(subjectLocal).(string)
		if err := a.Store.RevokeGrant(subject, c.Params("id")); err != nil {
			return c.SendStatus(404)
		}
		if logger.Instance != nil {
			logger.Instance.Infow("reader grant revoked", "subject", subject, "grant_id", c.Params("id"))
		}
		return c.SendStatus(204)
	}
}
