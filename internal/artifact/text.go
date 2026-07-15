package artifact

import (
	"fmt"
	"strings"
	"time"
)

// renderTextHeader renders the shared comment-style header written at
// the top of every text-format artifact: schema version, kind, source,
// and the query that produced it, plus a reminder that text is
// write-only (TZ.md section 4).
func renderTextHeader(h Header, query any) (string, error) {
	var b strings.Builder
	b.WriteString("# bogoslavAnalytics artifact -- write-only text format\n")
	b.WriteString("# not cached, and not accepted as a from_artifact input\n")
	b.WriteString("# use --format yaml or --format json to read this data back\n")
	b.WriteString("#\n")
	b.WriteString(fmt.Sprintf("# schema_version: %d\n", h.SchemaVersion))
	b.WriteString(fmt.Sprintf("# kind: %s\n", h.Kind))
	b.WriteString(fmt.Sprintf("# source.gitlab_url: %s\n", h.Source.GitlabURL))
	b.WriteString(fmt.Sprintf("# source.fetched_at: %s\n", h.Source.FetchedAt.Format(time.RFC3339)))
	b.WriteString("# query:\n")

	queryYAML, err := marshalYAML(query)
	if err != nil {
		return "", fmt.Errorf("render text header: %w", err)
	}
	for _, line := range strings.Split(strings.TrimRight(string(queryYAML), "\n"), "\n") {
		b.WriteString("#   " + line + "\n")
	}
	return b.String(), nil
}

// renderTextSection renders a labeled, indented YAML dump of v as
// plain, non-commented readable content below the header.
func renderTextSection(title string, v any) (string, error) {
	body, err := marshalYAML(v)
	if err != nil {
		return "", fmt.Errorf("render text section %s: %w", title, err)
	}
	return title + ":\n" + indentLines(string(body), "  "), nil
}

// indentLines prefixes every non-empty line of s with prefix.
func indentLines(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n") + "\n"
}
