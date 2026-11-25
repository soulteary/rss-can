# Go Report Card

**Grade: A+** (97.3%)

| Metric | Value |
| ------ | ----- |
| Files | 236 |
| Issues | 56 |

## Checks

| Check | Score |
| ----- | ----- |
| gofmt | 100% |
| go_vet | 100% |
| ineffassign | 100% |
| gocyclo | 77% |
| license | 100% |
| misspell | 99% |

## Issues

### gocyclo

- `internal/parser/csr_test.go`
  - Line 137: cyclomatic complexity 43 for function TestBrowserLifecycleAndLiveCache
- `internal/parser/field_expression_browser_test.go`
  - Line 19: cyclomatic complexity 34 for function TestLiveNativeCSRFieldExpressionsAndHookMetadata
- `internal/parser/csr.go`
  - Line 46: cyclomatic complexity 31 for function resolveBrowserURL
  - Line 435: cyclomatic complexity 20 for function browserData
  - Line 503: cyclomatic complexity 18 for function fetchBrowserData
  - Line 151: cyclomatic complexity 18 for function withBrowserPage
  - Line 362: cyclomatic complexity 17 for function ParsePageByBrowserContext
- `internal/rulefile/entries_test.go`
  - Line 16: cyclomatic complexity 28 for function TestDirectoryEntriesUseOpenedDescriptorAfterPathReplacement
- `internal/generator/generator.go`
  - Line 20: cyclomatic complexity 27 for function GenerateFeedsByTypeContext
- `internal/parser/repository_rules_test.go`
  - Line 21: cyclomatic complexity 22 for function TestRepositoryRulesExtractUpstreamSnapshots
- `internal/source/spec.go`
  - Line 66: cyclomatic complexity 20 for function (Spec).Validate
- `internal/rule/file_limits_test.go`
  - Line 18: cyclomatic complexity 20 for function TestRepositoryRejectsSpecialRulesWithoutLosingMetadata
- `internal/feedservice/snapshots.go`
  - Line 98: cyclomatic complexity 50 for function (*Snapshots).representation
- `internal/parser/ssr.go`
  - Line 118: cyclomatic complexity 49 for function ParseDataAndConfigBySSRContext
- `internal/access/store.go`
  - Line 156: cyclomatic complexity 36 for function validate
  - Line 341: cyclomatic complexity 20 for function (*Store).PutSource
  - Line 94: cyclomatic complexity 18 for function Open
  - Line 459: cyclomatic complexity 17 for function (*Store).CreateGrant
- `internal/access/config.go`
  - Line 47: cyclomatic complexity 21 for function (Config).Validate
- `internal/access/acl_preservation_test.go`
  - Line 13: cyclomatic complexity 20 for function TestACLUpdatesPreserveManagedContent
- `internal/cacher/instance_test.go`
  - Line 9: cyclomatic complexity 19 for function TestApplicationCachesKeepConfigurationAndValuesIndependent
- `internal/jssdk/sandbox_deno.go`
  - Line 200: cyclomatic complexity 18 for function (*denoWorker).executeContext
- `internal/parser/nuxt_document.go`
  - Line 112: cyclomatic complexity 17 for function parseNuxtCall
- `internal/server/sources_browser_test.go`
  - Line 23: cyclomatic complexity 41 for function TestSourceWizardBrowserCreatesCredentialWithoutExecutingContent
- `internal/jssdk/date_contract_test.go`
  - Line 60: cyclomatic complexity 22 for function TestDateContractRelativeUnitsAndLeapDays
- `internal/fn/link_test.go`
  - Line 10: cyclomatic complexity 22 for function TestLinkResolveRelative
- `internal/server/access.go`
  - Line 106: cyclomatic complexity 20 for function (*AccessController).requireFeed
- `internal/access/sources_test.go`
  - Line 15: cyclomatic complexity 20 for function TestManagedSourceOwnerRevisionAndRollback
- `internal/application/sources_test.go`
  - Line 16: cyclomatic complexity 18 for function TestManagedSnapshotsStayPinnedAcrossEdits
- `internal/parser/details_test.go`
  - Line 58: cyclomatic complexity 17 for function TestSSRDetailsRunConcurrentlyAndKeepOrder
- `internal/rule/cache_test.go`
  - Line 13: cyclomatic complexity 16 for function TestRulePathsAndReload
- `internal/parser/csr_postprocess.go`
  - Line 96: cyclomatic complexity 21 for function processCSRItemsContext
- `internal/parser/inspector_budget.go`
  - Line 122: cyclomatic complexity 16 for function readInspectorResponse
- `internal/parser/browser_requests.go`
  - Line 87: cyclomatic complexity 16 for function handlePausedRequestWithBudget
- `internal/generator/generator_test.go`
  - Line 16: cyclomatic complexity 16 for function TestUpstreamFeedsPreserveEntries
- `internal/access/inspect.go`
  - Line 13: cyclomatic complexity 16 for function Inspect
- `internal/server/feed.go`
  - Line 48: cyclomatic complexity 25 for function apiRSSWithService
- `internal/parser/browser_document_test.go`
  - Line 124: cyclomatic complexity 25 for function TestLiveBrowserMainDocumentStatusAndMetadata
- `internal/cmd/flags_test.go`
  - Line 11: cyclomatic complexity 22 for function TestApplyFlags
- `internal/feedservice/fair.go`
  - Line 32: cyclomatic complexity 21 for function (*fairAdmission).acquire
- `internal/rule/validation.go`
  - Line 16: cyclomatic complexity 20 for function ValidateConfig
- `internal/rule/repository_test.go`
  - Line 39: cyclomatic complexity 17 for function TestRepositoryReloadIsAtomicAndRetainsLastGood
- `internal/rule/config_cache.go`
  - Line 50: cyclomatic complexity 17 for function (*configStore).load
- `internal/server/access_test.go`
  - Line 136: cyclomatic complexity 16 for function TestTeamReaderGrantLifecycleAndFormatScope
- `internal/cacher/cacher_test.go`
  - Line 11: cyclomatic complexity 33 for function TestCacher
- `internal/network/request.go`
  - Line 159: cyclomatic complexity 31 for function getRemoteDocument
  - Line 35: cyclomatic complexity 19 for function HttpGetContext
- `scripts/bench-jssdk/main.go`
  - Line 28: cyclomatic complexity 28 for function main
- `internal/server/pages.go`
  - Line 104: cyclomatic complexity 21 for function (pageView).renderListPage
- `internal/server/admin.go`
  - Line 51: cyclomatic complexity 20 for function adminAccessStatus
- `internal/network/detail_urls.go`
  - Line 80: cyclomatic complexity 20 for function normalizeDetailURLs
- `internal/source/document.go`
  - Line 21: cyclomatic complexity 16 for function ValidateDocument
- `internal/feedservice/service.go`
  - Line 71: cyclomatic complexity 16 for function (*Service).Generate
- `internal/generator/sanitize.go`
  - Line 41: cyclomatic complexity 73 for function sanitizeFragment
  - Line 219: cyclomatic complexity 21 for function safeFeedLink
  - Line 258: cyclomatic complexity 16 for function plainFeedSummary
- `internal/parser/details.go`
  - Line 31: cyclomatic complexity 28 for function fillDetailsWithLinks
- `internal/access/store_test.go`
  - Line 34: cyclomatic complexity 19 for function TestStoreAuthorizationAndPersistence
- `internal/application/application.go`
  - Line 122: cyclomatic complexity 32 for function New
- `internal/application/fetch_contract_test.go`
  - Line 68: cyclomatic complexity 21 for function TestApplicationFinalBaseSurvivesFreshAndCachedFetch
- `internal/parser/inspector_budget_live_test.go`
  - Line 21: cyclomatic complexity 16 for function TestLiveInspectorInterceptionKeepsPolicyAndBoundedBodies
- `internal/parser/csr_postprocess_test.go`
  - Line 28: cyclomatic complexity 16 for function TestCSRPostprocessNormalizesAndFillsNativeHooks
- `internal/application/mode_contract_test.go`
  - Line 52: cyclomatic complexity 16 for function TestApplicationModesShareFieldAndEnrichmentContracts
- `internal/application/data_contract_test.go`
  - Line 16: cyclomatic complexity 16 for function TestApplicationPreservesCategoryAndRejectsInvalidDates

### misspell

- `internal/define/default.go`
  - Line 32: "THORTTLING" is a misspelling of "THROTTLING"
- `internal/jssdk/js/hep.js`
  - Line 36: "truely" is a misspelling of "truly"

---

_Generated by [Go Report Card](https://github.com/soulteary/goreportcard-action)._
