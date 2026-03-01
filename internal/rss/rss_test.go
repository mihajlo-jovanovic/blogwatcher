package rss

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

const sampleFeed = `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
<title>Example Feed</title>
<item>
<title>First</title>
<link>https://example.com/1</link>
<pubDate>Mon, 02 Jan 2006 15:04:05 GMT</pubDate>
</item>
<item>
<title>Second</title>
<link>https://example.com/2</link>
</item>
</channel>
</rss>`

func TestParseFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleFeed))
	}))
	defer server.Close()

	articles, err := ParseFeed(server.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("parse feed: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}
	if articles[0].PublishedDate == nil {
		t.Fatalf("expected published date")
	}
}

func TestDiscoverFeedURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><head><link rel="alternate" type="application/rss+xml" href="/feed.xml" /></head></html>`))
	})
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleFeed))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	feedURL, err := DiscoverFeedURL(server.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("discover feed: %v", err)
	}
	if feedURL == "" {
		t.Fatalf("expected feed url")
	}
}

func TestParseFeed_CappedMemoryOnHugePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?><rss version="2.0"><channel><title>Example Feed</title>`))
		chunk := []byte(strings.Repeat("<item><title>Item</title><link>https://example.com/</link></item>", 1000))
		for i := 0; i < 20000; i++ { // Would be ~1GB+ if unbound
			if _, err := w.Write(chunk); err != nil {
				break
			}
		}
	}))
	defer server.Close()

	// Ensure this completes quickly and doesn't run out of memory.
	articles, err := ParseFeed(server.URL, 2*time.Second)

	// Since we cleanly truncated mid-XML tag, the feed parser will error out
	// because it's invalid XML. This is the desired behavior for a maliciously huge feed.
	if err == nil {
		t.Fatalf("expected error due to truncated XML, but parsing succeeded with %d articles", len(articles))
	}
}
