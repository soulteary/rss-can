# Go Report Card

**Grade: A+** (97.7%)

| Metric | Value |
| ------ | ----- |
| Files | 177 |
| Issues | 37 |

## Checks

| Check | Score |
| ----- | ----- |
| go_vet | 100% |
| gofmt | 100% |
| ineffassign | 100% |
| gocyclo | 80% |
| license | 100% |
| misspell | 98% |

## Issues

### gocyclo

- `internal/cmd/flags_test.go`
  - Line 11: cyclomatic complexity 22 for function TestApplyFlags
- `internal/parser/csr_postprocess.go`
  - Line 96: cyclomatic complexity 21 for function processCSRItemsContext
- `internal/network/detail_urls.go`
  - Line 80: cyclomatic complexity 20 for function normalizeDetailURLs
- `internal/parser/csr_postprocess_test.go`
  - Line 28: cyclomatic complexity 16 for function TestCSRPostprocessNormalizesAndFillsNativeHooks
- `internal/parser/field_expression_browser_test.go`
  - Line 18: cyclomatic complexity 34 for function TestLiveNativeCSRFieldExpressionsAndHookMetadata
- `internal/cacher/cacher_test.go`
  - Line 11: cyclomatic complexity 33 for function TestCacher
- `internal/rule/file_limits_test.go`
  - Line 18: cyclomatic complexity 20 for function TestRepositoryRejectsSpecialRulesWithoutLosingMetadata
- `internal/cacher/instance_test.go`
  - Line 9: cyclomatic complexity 19 for function TestApplicationCachesKeepConfigurationAndValuesIndependent
- `internal/parser/details_test.go`
  - Line 58: cyclomatic complexity 17 for function TestSSRDetailsRunConcurrentlyAndKeepOrder
- `internal/server/admin.go`
  - Line 51: cyclomatic complexity 20 for function adminAccessStatus
- `internal/parser/inspector_budget_live_test.go`
  - Line 20: cyclomatic complexity 16 for function TestLiveInspectorInterceptionKeepsPolicyAndBoundedBodies
- `internal/parser/browser_requests.go`
  - Line 87: cyclomatic complexity 16 for function handlePausedRequestWithBudget
- `internal/generator/generator_test.go`
  - Line 16: cyclomatic complexity 16 for function TestUpstreamFeedsPreserveEntries
- `internal/application/fetch_contract_test.go`
  - Line 68: cyclomatic complexity 21 for function TestApplicationFinalBaseSurvivesFreshAndCachedFetch
- `internal/jssdk/date_contract_test.go`
  - Line 54: cyclomatic complexity 18 for function TestDateContractRelativeUnitsAndLeapDays
- `internal/parser/ssr.go`
  - Line 112: cyclomatic complexity 47 for function ParseDataAndConfigBySSRContext
- `scripts/bench-jssdk/main.go`
  - Line 28: cyclomatic complexity 28 for function main
- `internal/rulefile/entries_test.go`
  - Line 16: cyclomatic complexity 28 for function TestDirectoryEntriesUseOpenedDescriptorAfterPathReplacement
- `internal/parser/browser_document_test.go`
  - Line 124: cyclomatic complexity 25 for function TestLiveBrowserMainDocumentStatusAndMetadata
- `internal/generator/generator.go`
  - Line 19: cyclomatic complexity 20 for function GenerateFeedsByTypeContext
- `internal/server/pages.go`
  - Line 99: cyclomatic complexity 17 for function (pageView).renderListPage
- `internal/rule/cache_test.go`
  - Line 13: cyclomatic complexity 16 for function TestRulePathsAndReload
- `internal/parser/csr_test.go`
  - Line 136: cyclomatic complexity 43 for function TestBrowserLifecycleAndLiveCache
- `internal/parser/csr.go`
  - Line 46: cyclomatic complexity 31 for function resolveBrowserURL
  - Line 490: cyclomatic complexity 18 for function fetchBrowserData
  - Line 429: cyclomatic complexity 18 for function browserData
  - Line 151: cyclomatic complexity 17 for function withBrowserPage
  - Line 359: cyclomatic complexity 16 for function ParsePageByBrowserContext
- `internal/network/request.go`
  - Line 152: cyclomatic complexity 27 for function getRemoteDocument
  - Line 36: cyclomatic complexity 18 for function HttpGetContext
- `internal/fn/link_test.go`
  - Line 10: cyclomatic complexity 22 for function TestLinkResolveRelative
- `internal/rule/config_cache.go`
  - Line 50: cyclomatic complexity 17 for function (*configStore).load
- `internal/parser/inspector_budget.go`
  - Line 122: cyclomatic complexity 16 for function readInspectorResponse
- `internal/application/mode_contract_test.go`
  - Line 50: cyclomatic complexity 16 for function TestApplicationModesShareFieldAndEnrichmentContracts
- `internal/application/data_contract_test.go`
  - Line 16: cyclomatic complexity 16 for function TestApplicationPreservesCategoryAndRejectsInvalidDates
- `internal/rule/validation.go`
  - Line 16: cyclomatic complexity 20 for function ValidateConfig
- `internal/rule/repository_test.go`
  - Line 39: cyclomatic complexity 17 for function TestRepositoryReloadIsAtomicAndRetainsLastGood
- `internal/server/feed.go`
  - Line 44: cyclomatic complexity 16 for function apiRSSWithService
- `internal/parser/details.go`
  - Line 31: cyclomatic complexity 28 for function fillDetailsWithLinks
- `internal/jssdk/sandbox_deno.go`
  - Line 188: cyclomatic complexity 18 for function (*denoWorker).executeContext

### misspell

- `internal/define/default.go`
  - Line 32: "THORTTLING" is a misspelling of "THROTTLING"
- `internal/jssdk/js/hep.js`
  - Line 36: "truely" is a misspelling of "truly"

---

_Generated by [Go Report Card](https://github.com/soulteary/goreportcard-action)._
