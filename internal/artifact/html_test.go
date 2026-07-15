package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestWrite_producesHTMLForAllKinds checks that FormatHTML produces a
// non-empty, self-contained page for every one of the four artifact
// kinds. "Self-contained" is checked structurally: no <link> (external
// stylesheet), no <script> tag at all (none is needed), and no
// @import/@font-face pulling in a remote resource -- the CSS lives
// entirely in the page's own <style> block (per the html format's
// requirements: single file, no external requests).
func TestWrite_producesHTMLForAllKinds(t *testing.T) {
	for _, k := range allKindCases() {
		t.Run(k.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), k.name+"_test.html")
			if err := k.write(FormatHTML, path); err != nil {
				t.Fatalf("write() error = %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read written file: %v", err)
			}
			html := string(data)

			if len(html) == 0 {
				t.Fatal("html output is empty")
			}
			if !strings.Contains(html, "<style>") {
				t.Error("html output has no inline <style> block")
			}
			if !strings.Contains(html, k.name) {
				t.Errorf("html output does not mention kind %q", k.name)
			}
			if strings.Contains(html, "<link") {
				t.Error("html output contains a <link> tag: not self-contained")
			}
			if strings.Contains(html, "<script") {
				t.Error("html output contains a <script> tag: none is needed")
			}
			if strings.Contains(html, "@import") {
				t.Error("html output contains @import: not self-contained")
			}
		})
	}
}

// TestRead_rejectsHTMLFormat checks, for all four artifact kinds, that
// reading a written .html file fails with the same ErrNotReadable
// sentinel used for text: html is write-only presentation, same as
// text (TZ.md section 4).
func TestRead_rejectsHTMLFormat(t *testing.T) {
	for _, k := range allKindCases() {
		t.Run(k.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), k.name+"_test.html")
			if err := k.write(FormatHTML, path); err != nil {
				t.Fatalf("write() error = %v", err)
			}

			err := k.read(path)
			if !errors.Is(err, ErrNotReadable) {
				t.Errorf("read() error = %v, want ErrNotReadable", err)
			}
		})
	}
}

// TestWriteCommentList_htmlEscapesUntrustedCommentBody is the
// important escaping test: comment bodies are arbitrary text authored
// by GitLab users and must never be injected into the page unescaped.
// If html/template were swapped for text/template, the raw payloads
// would appear verbatim and this test would fail on the
// strings.Contains(html, raw) assertions below.
func TestWriteCommentList_htmlEscapesUntrustedCommentBody(t *testing.T) {
	const scriptPayload = `<script>alert(1)</script>`
	const imgPayload = `<img src=x onerror=alert(1)>`

	doc := sampleCommentList()
	doc.Items[0].Body = scriptPayload
	doc.Items[1].Body = imgPayload

	path := filepath.Join(t.TempDir(), "comment_list_xss_test.html")
	if err := WriteCommentList(doc, FormatHTML, path); err != nil {
		t.Fatalf("WriteCommentList() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	html := string(data)

	if strings.Contains(html, scriptPayload) {
		t.Errorf("html output contains the raw <script> payload unescaped:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("html output does not contain the escaped form of the <script> payload")
	}

	if strings.Contains(html, imgPayload) {
		t.Errorf("html output contains the raw <img onerror=...> payload unescaped:\n%s", html)
	}
	if !strings.Contains(html, "&lt;img src=x onerror=alert(1)&gt;") {
		t.Error("html output does not contain the escaped form of the <img onerror=...> payload")
	}
}

// TestWriteLabeledComments_htmlEscapesUntrustedCommentBody repeats the
// escaping check on the labeled_comments content template, which
// groups by label rather than by MR and is therefore a distinct code
// path from comment_list's.
func TestWriteLabeledComments_htmlEscapesUntrustedCommentBody(t *testing.T) {
	const payload = `<script>alert(1)</script>`

	doc := sampleLabeledComments()
	doc.Items[0].Body = payload

	path := filepath.Join(t.TempDir(), "labeled_comments_xss_test.html")
	if err := WriteLabeledComments(doc, FormatHTML, path); err != nil {
		t.Fatalf("WriteLabeledComments() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	html := string(data)

	if strings.Contains(html, payload) {
		t.Errorf("html output contains the raw <script> payload unescaped:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("html output does not contain the escaped form of the <script> payload")
	}
}

// TestWriteMRList_htmlNeutralizesJavascriptURL checks what html/template
// actually does with a javascript: URL landing in an href attribute:
// it does not pass the scheme through, it replaces the whole value with
// the sentinel "#ZgotmplZ" (html/template's documented behavior for
// content it judges unsafe for a URL context). The raw "javascript:"
// scheme must never reach the href attribute.
func TestWriteMRList_htmlNeutralizesJavascriptURL(t *testing.T) {
	doc := sampleMRList()
	doc.Items[0].Title = "evil link"
	doc.Items[0].WebURL = "javascript:alert(1)"

	path := filepath.Join(t.TempDir(), "mr_list_jsurl_test.html")
	if err := WriteMRList(doc, FormatHTML, path); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	html := string(data)

	if strings.Contains(html, `href="javascript:`) {
		t.Errorf("html output has a live javascript: href:\n%s", html)
	}
	if !strings.Contains(html, `href="#ZgotmplZ"`) {
		t.Error(`html output does not neutralize the javascript: URL to href="#ZgotmplZ"`)
	}
}

// TestWriteLabeledComments_htmlShowsClassifierProminently checks that
// the classifier provenance block (tool, model, taxonomy_version,
// classified_at) is rendered in its own, clearly marked section next
// to the query, not mixed in among the item rows (TZ.md section 8.3).
func TestWriteLabeledComments_htmlShowsClassifierProminently(t *testing.T) {
	doc := sampleLabeledComments()
	path := filepath.Join(t.TempDir(), "labeled_comments_provenance_test.html")
	if err := WriteLabeledComments(doc, FormatHTML, path); err != nil {
		t.Fatalf("WriteLabeledComments() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	html := string(data)

	if !strings.Contains(html, `class="provenance"`) {
		t.Fatal("html output has no dedicated provenance section")
	}

	// The provenance section must appear before the item content, and
	// must carry all four classifier fields.
	provenanceStart := strings.Index(html, `class="provenance"`)
	contentStart := strings.Index(html, `class="content"`)
	if provenanceStart < 0 || contentStart < 0 || provenanceStart > contentStart {
		t.Errorf("provenance section (at %d) is not shown before item content (at %d)", provenanceStart, contentStart)
	}

	for _, want := range []string{"opencode", "glm-5.2", "3", doc.Classifier.ClassifiedAt.Format(time.RFC3339)} {
		if !strings.Contains(html, want) {
			t.Errorf("html output does not mention classifier field %q", want)
		}
	}
}

// TestWriteFilteredComments_htmlShowsFilteredLabels checks that the
// labels a filtered_comments artifact was narrowed by are visible in
// its query section.
func TestWriteFilteredComments_htmlShowsFilteredLabels(t *testing.T) {
	doc := sampleFilteredComments()
	path := filepath.Join(t.TempDir(), "filtered_comments_labels_test.html")
	if err := WriteFilteredComments(doc, FormatHTML, path); err != nil {
		t.Fatalf("WriteFilteredComments() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	html := string(data)

	for _, label := range doc.Query.Labels {
		if !strings.Contains(html, label) {
			t.Errorf("html output does not mention filtered label %q", label)
		}
	}
}

// TestWriteMRList_htmlShowsStrategyAndSmokeProminently checks that the
// mr_list query section surfaces more_than, strategy, and smoke, so a
// reader can tell whether the counts came from the events or
// bruteforce strategy (TZ.md section 5).
func TestWriteMRList_htmlShowsStrategyAndSmokeProminently(t *testing.T) {
	doc := sampleMRList()
	path := filepath.Join(t.TempDir(), "mr_list_strategy_test.html")
	if err := WriteMRList(doc, FormatHTML, path); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	html := string(data)

	for _, want := range []string{"more_than", "5", "strategy", string(doc.Query.Strategy), "smoke", string(doc.Query.Smoke)} {
		if !strings.Contains(html, want) {
			t.Errorf("html output does not mention %q", want)
		}
	}
}
