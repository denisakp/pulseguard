package notifier

import "testing"

func TestGenerateOperatorEmailHTML(t *testing.T) {
	n := &SMTPNotifier{}
	html := n.generateOperatorEmailHTML(&OperatorNotification{
		Title: "2 hosts went offline",
		Body:  "The following hosts stopped reporting:",
		Items: []string{"web-1", "db-2 <prod>"},
	})

	for _, want := range []string{
		"2 hosts went offline", // title
		"stopped reporting",    // body
		"web-1",                // item
		"db-2 &lt;prod&gt;",    // item, HTML-escaped
		"<ul",                  // list rendered
	} {
		if !contains(html, want) {
			t.Errorf("operator email HTML missing %q\n---\n%s", want, html)
		}
	}
}

func TestGenerateOperatorEmailHTML_NoItems(t *testing.T) {
	n := &SMTPNotifier{}
	html := n.generateOperatorEmailHTML(&OperatorNotification{Title: "Digest", Body: "Nothing listed"})
	if contains(html, "<ul") {
		t.Errorf("expected no list when Items empty, got:\n%s", html)
	}
	if !contains(html, "Digest") || !contains(html, "Nothing listed") {
		t.Errorf("missing title/body:\n%s", html)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
