package rule

import (
	"fmt"
	"strings"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/limits"
	"github.com/soulteary/RSS-Can/internal/parser"
	"github.com/soulteary/cascadia"
)

// ValidateConfig is the executable rule contract, separate from JSON decoding.
// CSR may override GetData and therefore need not use the native SDK's Title
// and Link expressions. It still declares its HTTP source and CSS page scope.
func ValidateConfig(config define.JavaScriptConfig) error {
	mode := strings.ToLower(strings.TrimSpace(config.Mode))
	if mode == "" {
		mode = define.DEFAULT_PARSE_MODE
	}
	switch mode {
	case define.PARSE_MODE_SSR:
		if strings.TrimSpace(config.ReadySelector) != "" {
			return fmt.Errorf("ReadySelector is supported only for CSR/MIX parsing")
		}
		if err := parser.ValidateSSRConfig(config); err != nil {
			return err
		}
	case define.PARSE_MODE_MIX:
		if err := parser.ValidateSSRConfig(config); err != nil {
			return err
		}
	case define.PARSE_MODE_CSR:
		if err := parser.ValidatePageConfig(config); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported Mode %q", config.Mode)
	}
	if _, err := config.CompileIDRegexp(); err != nil {
		return err
	}
	if config.PagerLimit < 0 || config.PagerLimit > limits.MaxPages {
		return fmt.Errorf("PagerLimit must be between 0 and %d", limits.MaxPages)
	}
	if config.Pager != "" {
		if _, err := cascadia.Compile(config.Pager); err != nil {
			return fmt.Errorf("invalid Pager selector: %w", err)
		}
	}
	for _, field := range []struct {
		name string
		hook define.ConfigHook
	}{
		{"DateTimeHook", config.DateTimeHook}, {"CategoryHook", config.CategoryHook}, {"DescriptionHook", config.DescriptionHook}, {"ContentHook", config.ContentHook},
	} {
		action := strings.TrimSpace(field.hook.Action)
		if action != "" && action != define.ConfigHookReadLink {
			return fmt.Errorf("unsupported %s action %q", field.name, action)
		}
		// Empty URL selects the current item anchor; empty Object selects body.
		if action == define.ConfigHookReadLink && strings.TrimSpace(field.hook.Object) != "" {
			if _, err := cascadia.Compile(field.hook.Object); err != nil {
				return fmt.Errorf("invalid %s Object selector: %w", field.name, err)
			}
		}
	}
	return validateFieldExpressions(config)
}

// Explicit selector expressions fail before browser/HTTP work rather than
// becoming an empty field. Legacy unprefixed strings keep their existing DSL.
func validateFieldExpressions(config define.JavaScriptConfig) error {
	for _, field := range []struct{ name, value string }{
		{"Title", config.Title}, {"Author", config.Author}, {"Category", config.Category}, {"DateTime", config.DateTime}, {"Description", config.Description}, {"Link", config.Link},
		{"DateTimeHook.URL", config.DateTimeHook.URL}, {"CategoryHook.URL", config.CategoryHook.URL}, {"DescriptionHook.URL", config.DescriptionHook.URL}, {"ContentHook.URL", config.ContentHook.URL},
	} {
		expression := define.ParseFieldExpression(field.value)
		if !expression.Explicit || !expression.Selector {
			continue
		}
		if expression.Value == "" {
			return fmt.Errorf("%s css: selector must not be empty", field.name)
		}
		if _, err := cascadia.Compile(expression.Value); err != nil {
			return fmt.Errorf("invalid %s css: selector: %w", field.name, err)
		}
	}
	return nil
}
