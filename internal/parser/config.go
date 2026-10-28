package parser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/cascadia"
)

// ValidatePageConfig checks the common source contract before work is started.
// Rules may intentionally access private HTTP(S) origins; this is not the
// public-only inspector policy.
func ValidatePageConfig(config define.JavaScriptConfig) error {
	target, err := url.Parse(strings.TrimSpace(config.URL))
	if err != nil || target.Hostname() == "" || (!strings.EqualFold(target.Scheme, "http") && !strings.EqualFold(target.Scheme, "https")) {
		return fmt.Errorf("URL must be an absolute HTTP(S) URL")
	}
	if err := validateScope(config.ListContainer); err != nil {
		return err
	}
	return ValidateReadySelector(config.ReadySelector)
}

func validateScope(scope string) error {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return fmt.Errorf("ListContainer is required")
	}
	if _, err := cascadia.Compile(scope); err != nil {
		return fmt.Errorf("invalid ListContainer selector: %w", err)
	}
	return nil
}

// ValidateSSRConfig also requires a title expression. Field expressions such as
// Title may be literal values, so they are not all treated as CSS selectors.
func ValidateSSRConfig(config define.JavaScriptConfig) error {
	if err := ValidatePageConfig(config); err != nil {
		return err
	}
	return validateTitle(config.Title)
}

func validateSSRFields(config define.JavaScriptConfig) error {
	if err := validateScope(config.ListContainer); err != nil {
		return err
	}
	return validateTitle(config.Title)
}
func validateTitle(title string) error {
	if strings.TrimSpace(define.ParseFieldExpression(title).Value) == "" {
		return fmt.Errorf("Title is required for SSR/MIX parsing")
	}
	return nil
}

// ValidateReadySelector checks the optional browser readiness expression. Empty
// retains the historical wait for ListContainer; an explicit marker may become
// ready even when the data scope contains no entries.
func ValidateReadySelector(selector string) error {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil
	}
	if _, err := cascadia.Compile(selector); err != nil {
		return fmt.Errorf("invalid ReadySelector selector: %w", err)
	}
	return nil
}
