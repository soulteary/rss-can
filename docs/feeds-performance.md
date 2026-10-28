# feeds fork performance comparison

Reviewed on 2026-10-28 with Go 1.27.3.

This document records the local comparison used when replacing
`github.com/gorilla/feeds` v1.2.0 with `github.com/soulteary/feeds` v1.3.0. The
fork contains an optimized custom XML extension path, but that path does not
exist in the original module. The shared RSS, Atom and JSON APIs must therefore
be measured separately from the extension-specific optimization.

## Test environment

- CPU: Apple M5
- OS and architecture: macOS (`darwin/arm64`)
- Go: 1.27.3
- `GOMAXPROCS=1` and `-cpu=1`
- Samples: five per format and item count
- Minimum benchmark duration: 300 ms per sample
- Original module: `github.com/gorilla/feeds` v1.2.0
- Fork module: `github.com/soulteary/feeds` v1.3.0

Both implementations ran in one comparison module with the same preconstructed
feeds containing 10, 100 or 1,000 items. Timing covers conversion, serialization
and returned string allocation. Reported values are medians; lower is better.

## Shared-format latency

| Format | Items | Original median | Fork median | Fork latency change |
| --- | ---: | ---: | ---: | ---: |
| Atom | 10 | 13.997 µs/op | 19.851 µs/op | +41.8% |
| RSS | 10 | 13.024 µs/op | 18.886 µs/op | +45.0% |
| JSON | 10 | 5.823 µs/op | 8.652 µs/op | +48.6% |
| Atom | 100 | 128.212 µs/op | 194.588 µs/op | +51.8% |
| RSS | 100 | 112.025 µs/op | 152.834 µs/op | +36.4% |
| JSON | 100 | 53.173 µs/op | 75.390 µs/op | +41.8% |
| Atom | 1,000 | 1.431 ms/op | 1.936 ms/op | +35.3% |
| RSS | 1,000 | 1.291 ms/op | 1.833 ms/op | +42.0% |
| JSON | 1,000 | 0.538 ms/op | 0.870 ms/op | +61.8% |

For the shared plain-feed workloads, v1.3.0 was 35–62% slower on this machine.
This comparison does not support a general throughput-improvement claim for the
fork.

## Shared-format memory

| Format | Items | Fork bytes change | Fork allocation-count change |
| --- | ---: | ---: | ---: |
| Atom | 10 | +84.9% | +52.6% |
| RSS | 10 | +77.1% | +76.2% |
| JSON | 10 | +33.0% | +78.9% |
| Atom | 100 | +63.6% | +54.8% |
| RSS | 100 | +73.0% | +85.4% |
| JSON | 100 | +29.6% | +93.8% |
| Atom | 1,000 | +55.2% | +55.4% |
| RSS | 1,000 | +33.0% | +87.2% |
| JSON | 1,000 | +29.5% | +99.2% |

The fork also allocated more memory on these shared workloads. Some additional
work and output is intentional: the fork supports more metadata and applies
wire-format corrections that the original does not perform.

## Extension-specific optimization

The fork's documented performance optimization targets custom XML extensions.
For its 100-entry extension fixture, the fork's own before/after measurements
report approximately:

- allocated bytes: 1,503,615 to 881,208 B/op, 41.4% lower;
- allocations: 6,340 to 3,440 allocs/op, 45.7% fewer.

Those numbers compare two revisions inside the fork. They are not a comparison
with gorilla/feeds v1.2.0 because the original module has no equivalent generic
XML extension API. The fork does not claim a fixed extension throughput gain;
its documentation identifies noisy timing even on unchanged control paths.

## Output compatibility

The same input does not always produce byte-identical output. Relevant
intentional corrections include:

- Atom uses an explicit `Feed.Id` instead of always falling back to the primary
  link.
- Atom emits `published` from `Item.Created`.
- RSS XML namespace serialization can place declarations differently.
- RSS author values use valid email-address semantics; name-only authors remain
  available in Atom and JSON.
- JSON Feed uses corrected standard field names for affected metadata.

RSS Can uses keyed struct literals, so the additive fields do not create a
source compatibility problem. Consumers that compare exact XML bytes should be
reviewed separately; semantic XML/JSON parsing is preferred.

## Reproduction

Use a temporary comparison module that imports both module paths, constructs
equivalent feeds before timing and invokes `ToAtom`, `ToRss` and `ToJSON`:

```sh
GOMAXPROCS=1 go test \
  -run '^$' \
  -bench 'Benchmark(Original|Fork)$' \
  -benchmem \
  -benchtime=300ms \
  -count=5 \
  -cpu=1
```

For smaller differences, increase the sample count, alternate process order and
compare saved output with `benchstat`. Keep the Go version, CPU power state,
fixtures and process settings unchanged.

## Relevance to RSS Can

RSS Can currently uses plain RSS, Atom and JSON generation without custom XML
extensions. The shared-format benchmark is therefore more representative than
the fork's extension-specific fast path. The migration provides specification
corrections, additional metadata support and a maintained fork, but it should
not be justified as a present-day performance optimization for RSS Can.

If RSS Can later emits custom XML extensions, the fork's lower allocation path
may reduce GC pressure. An application-level follow-up should benchmark
`GenerateFeedsByType` with realistic item counts, author/date combinations and
content sizes while validating parsed feed semantics rather than raw bytes.
