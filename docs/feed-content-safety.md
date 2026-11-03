# Feed content safety

v0.5.0 and later, and current source builds, sanitize each item's HTML description and content before writing
RSS, Atom or JSON Feed. XML/JSON escaping alone does not make embedded HTML safe
when a reader renders it. This applies to trusted legacy rules as well as team
sources; there is no per-rule bypass.

The allowlist retains article text, headings, emphasis, paragraphs, lists, code,
quotations, basic tables and links. Script/style blocks, SVG/MathML, frames,
embedded objects, templates, audio/video and other active subtrees are removed.
Event handlers, CSS, classes, IDs and unsupported attributes are removed. Links
accept HTTP(S), relative web addresses resolved against the source URL, and
`mailto:`. Article URLs themselves accept only HTTP(S). Credentials in URLs and
non-web schemes such as `javascript:`, `data:`, `file:` and `vbscript:` are rejected.
Anchors include `rel="noopener noreferrer"`.
JSON Feed's `summary` remains plain text: sanitized markup is reduced to text
and entities are decoded, so readers do not display literal tags or entities.

Images are replaced by their escaped alternative text. Image URLs, `srcset`,
CSS URLs and other automatically loaded resources are not retained: opening a
feed should not silently contact a source's tracking server. The sanitizer does
not fetch images and does not create an image proxy. Markdown-looking text is
treated as text, not converted to HTML; if a third-party reader chooses to render
that text as Markdown, its additional rendering behavior is outside this filter.
Following an allowed link is still a user navigation and can disclose the user's
IP address to the destination; URL tracking parameters are not removed.

This intentionally changes output for rules relying on custom styling, forms,
interactive embeds or inline images. Their article text and safe links remain.
Sanitized text/attribute expansion and nesting are bounded, and cancellation is
checked while scanning. Sanitization failures return a generation error rather
than partially publishing a feed. The output's total size limits still apply.

Run `go test -race ./internal/generator` for the regression corpus, and
`go test ./internal/generator -run '^$' -fuzz FuzzFeedHTMLSanitizer -fuzztime=10s`
for mutation testing. The corpus includes encoded schemes, malformed and nested
active markup, attribute injection, resource tracking, output budgets and all
three feed formats.
