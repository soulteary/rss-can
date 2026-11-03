package application

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/soulteary/RSS-Can/internal/define"
	"github.com/soulteary/RSS-Can/internal/generator"
	"github.com/soulteary/RSS-Can/internal/network"
	"github.com/soulteary/RSS-Can/internal/rule"
	"github.com/soulteary/RSS-Can/internal/source"
)

func (a *App) lookupSource(id string) (define.RuleCache, bool) {
	if a.access != nil {
		if src, ok := a.access.Managed(id); ok {
			body, err := src.Spec.Bytes()
			if err != nil {
				return define.RuleCache{}, false
			}
			return define.RuleCache{Managed: true, Body: body, File: "managed/" + id, Sign: fmt.Sprintf("%x", sha256.Sum256(body))}, true
		}
	}
	return a.rules.Lookup(id)
}

func (a *App) configureSource(ctx context.Context, snapshot define.RuleCache) (define.JavaScriptConfig, error) {
	if !snapshot.Managed {
		return a.rules.Configure(a.resources(ctx), snapshot)
	}
	spec, err := source.Parse(snapshot.Body)
	if err != nil {
		return define.JavaScriptConfig{}, err
	}
	config, err := spec.Config()
	config.File = snapshot.File
	return config, err
}

func (a *App) sourceSnapshots() map[string]define.RuleCache {
	result := a.rules.Snapshot()
	if a.access != nil {
		for _, src := range a.access.ManagedSources() {
			if snap, ok := a.lookupSource(src.ID); ok {
				result[src.ID] = snap
			}
		}
	}
	return result
}

func (a *App) sourceMetadata() map[string]rule.Metadata {
	result := a.rules.SnapshotMetadata()
	if a.access != nil {
		for _, src := range a.access.ManagedSources() {
			if snap, ok := a.lookupSource(src.ID); ok {
				result[src.ID] = rule.Metadata{Name: src.Spec.Name, File: snap.File, Sign: snap.Sign}
			}
		}
	}
	return result
}

func (a *App) previewSource(ctx context.Context, spec source.Spec) ([]define.InfoItem, error) {
	config, err := spec.Config()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, min(a.config.Server.Timeout, 8*time.Second))
	defer cancel()
	ctx = network.WithFetchPolicy(network.WithFetchBudget(a.resources(ctx)), network.PublicPolicy())
	config.DisableCache = true
	data := rule.GetWebsiteDataWithConfigContext(ctx, config)
	if data.Code != define.ERROR_CODE_NULL {
		return nil, fmt.Errorf("source preview failed")
	}
	if err := source.ValidateItems(data.Body); err != nil {
		return nil, err
	}
	if _, err := generator.GenerateFeedsByTypeContext(ctx, config, data, define.FEED_TYPE_RSS); err != nil {
		return nil, err
	}
	return data.Body[:min(len(data.Body), 5)], nil
}
