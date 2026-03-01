package scraper

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Hyaxia/blogwatcher/internal/safeclient"
)

func TestMain(m *testing.M) {
	safeclient.SetTestAllowPrivate(true)
	os.Exit(m.Run())
}

func TestScrapeBlog(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
  <article><h2><a href="/one">First</a></h2></article>
  <article><h2><a href="/one">First Duplicate</a></h2></article>
  <div class="post"><h3><span><a href="/two" title="Second">Ignore Text</a></span></h3></div>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	articles, err := ScrapeBlog(server.URL, "article h2 a, .post", 2*time.Second)
	if err != nil {
		t.Fatalf("scrape blog: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}

	if articles[0].URL == "" || articles[1].URL == "" {
		t.Fatalf("expected URLs")
	}
}

func TestScrapeBlog_CappedMemoryOnHugePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>"))
		chunk := []byte(strings.Repeat("<article><h2><a href='/x'>X</a></h2></article>", 1000))
		for i := 0; i < 10000; i++ { // Would be ~500MB+ if unbound
			if _, err := w.Write(chunk); err != nil {
				break
			}
		}
	}))
	defer server.Close()

	// Ensure this completes quickly and doesn't run out of memory.
	articles, err := ScrapeBlog(server.URL, "article h2 a", 2*time.Second)
	if err != nil {
		t.Fatalf("scrape blog on huge payload failed: %v", err)
	}

	// Because of MaxBodySize (5MB), it won't parse the full 500MB payload.
	// We just ensure it found SOME articles and returned successfully.
	if cap := 5000; len(articles) > cap {
		t.Fatalf("parsed suspiciously many articles (%d), bounds might not be working", len(articles))
	}
	if len(articles) == 0 {
		t.Fatalf("expected some articles to be parsed even if truncated")
	}
}
