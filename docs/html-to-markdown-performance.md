# html-to-markdown fork performance comparison

Reviewed on 2026-10-28 with Go 1.27.3.

This document records the local comparison used when replacing
`github.com/JohannesKaufmann/html-to-markdown/v2` v2.5.2 with
`github.com/soulteary/html-to-markdown/v2` v2.0.0. It is a reproducible baseline
for dependency reviews, not a performance guarantee for every HTML document.

RSS Can subsequently upgraded the fork to v2.0.1. The tables below remain the
original v2.0.0 migration baseline. Version 2.0.1 adds DOM traversal and text
buffer optimizations; it was not substituted into these historical results.

## Test environment

- CPU: Apple M5
- OS and architecture: macOS (`darwin/arm64`)
- Go: 1.27.3
- Benchmark workers: 10 (`Benchmark...-10`)
- Samples: five per workload and mode
- Minimum benchmark duration: 250 ms per sample
- Original module: `github.com/JohannesKaufmann/html-to-markdown/v2` v2.5.2
- Fork module: `github.com/soulteary/html-to-markdown/v2` v2.0.0

Both implementations ran in one comparison module with identical generated
inputs. Before timing, the harness verified byte-for-byte identical Markdown
output for all six workloads. Reported values are medians; lower values are
better.

## Results

`Fresh` constructs and configures a converter for every operation. `Reuse`
constructs one converter outside the timed loop while still parsing a new HTML
document for every operation.

### Latency

| Workload | Mode | Original median | Fork median | Latency reduction |
| --- | --- | ---: | ---: | ---: |
| Small fragment | Fresh | 6.384 µs/op | 7.419 µs/op | -16.2% |
| Small fragment | Reuse | 4.785 µs/op | 4.385 µs/op | 8.4% |
| Article sections | Fresh | 761.964 µs/op | 672.675 µs/op | 11.7% |
| Article sections | Reuse | 761.371 µs/op | 664.889 µs/op | 12.7% |
| Markdown punctuation | Fresh | 1.417 ms/op | 0.687 ms/op | 51.5% |
| Markdown punctuation | Reuse | 1.416 ms/op | 0.673 ms/op | 52.4% |
| Links and images | Fresh | 565.649 µs/op | 577.156 µs/op | -2.0% |
| Links and images | Reuse | 567.713 µs/op | 569.023 µs/op | -0.2% |
| Many sibling nodes | Fresh | 1.148 ms/op | 1.121 ms/op | 2.3% |
| Many sibling nodes | Reuse | 1.150 ms/op | 1.124 ms/op | 2.2% |
| Formatted table | Fresh | 448.999 µs/op | 437.194 µs/op | 2.6% |
| Formatted table | Reuse | 450.022 µs/op | 430.408 µs/op | 4.4% |

A negative reduction means that the fork was slower in that sample. The fork is
approximately 2.1 times faster for dense Markdown punctuation and 12% faster
for repeated article sections. Links are effectively equal. Creating a new
converter dominates the tiny fragment, where the fork was 16% slower; reusing a
converter made the fork 8% faster.

### Memory and allocations

The fork's more consistent benefit is lower allocation pressure on non-trivial
documents.

| Workload | Mode | Bytes reduction | Allocation reduction |
| --- | --- | ---: | ---: |
| Small fragment | Fresh | -1.0% | 4.0% |
| Small fragment | Reuse | 10.5% | 24.3% |
| Article sections | Fresh | 13.5% | 34.4% |
| Article sections | Reuse | 13.8% | 34.7% |
| Markdown punctuation | Fresh | 72.6% | 99.5% |
| Markdown punctuation | Reuse | 72.7% | 99.7% |
| Links and images | Fresh | 1.1% | 6.1% |
| Links and images | Reuse | 1.2% | 6.3% |
| Many sibling nodes | Fresh | 15.0% | 36.4% |
| Many sibling nodes | Reuse | 15.2% | 36.6% |
| Formatted table | Fresh | 14.3% | 31.6% |
| Formatted table | Reuse | 14.5% | 31.9% |

The largest change is the Markdown punctuation workload: reuse mode falls from
2,904,209 to 791,731 B/op and from 32,111 to 94 allocs/op. Article, many-node
and table workloads reduce allocations by roughly one third.

## Workloads

- Small fragment: one heading and one short formatted paragraph.
- Article sections: 200 repeated sections containing headings and formatted
  prose.
- Markdown punctuation: 500 repetitions of Unicode text and Markdown-sensitive
  punctuation.
- Links and images: 200 paragraphs containing an absolute link and relative
  image.
- Many sibling nodes: 2,000 span elements under one parent.
- Formatted table: 200 rows with strong text and numeric cells.

These workloads include HTML parsing, DOM normalization, escaping, rendering
and output allocation. They do not model network access or the rest of the RSS
Can feed pipeline.

## Reproduction

Use a temporary comparison module that imports both module paths and feeds the
same generated strings to each implementation. Verify equality before timing:

```sh
go test -run TestOutputsMatch
go test \
  -run '^$' \
  -bench 'Benchmark(Upstream|Fork)$' \
  -benchmem \
  -benchtime=250ms \
  -count=5
```

For more statistically rigorous release comparisons, alternate implementation
order between processes, increase the sample count to ten or more, save the raw
Go benchmark output, and use `benchstat`. Keep the Go version, CPU power state,
input corpus and process settings unchanged.

## Relevance to RSS Can

RSS Can currently calls the root `ConvertString` helper for each conversion, so
the `Fresh` results most closely represent existing behavior. Typical article
content should benefit from lower allocation pressure and may see a low-double-
digit latency improvement. Content dominated by escaped Markdown punctuation
can improve substantially; very small fragments or link-heavy pages should not
be assumed faster.

A future application benchmark should exercise `Html2Md` with captured,
sanitized article bodies from representative feeds. If conversion becomes a
measured hotspot, RSS Can can also retain a configured converter and use the
`Reuse` path, provided plugin registration is completed before concurrent use.
