package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestCategoryCrawlerDiscoversCardsAcrossPagesAndCategories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Query().Get("page") {
		case "1":
			_, _ = writer.Write([]byte(`<a href="/alpha">Alpha</a><a href="/affiliate-networks/gambling">Category</a><a href="https://other.example/beta">Other</a>`))
		case "2":
			_, _ = writer.Write([]byte(`<a href="/beta?ref=directory">Beta</a><a href="/alpha">Duplicate</a>`))
		default:
			_, _ = writer.Write([]byte(`<html><body>No cards</body></html>`))
		}
	}))
	defer server.Close()

	crawler := NewCategoryCrawler(ScraperConfig{Timeout: time.Second, UserAgent: "test-agent"})
	got, err := crawler.DiscoverCardURLs(context.Background(), []string{server.URL + "/affiliate-networks/gambling", server.URL + "/affiliate-networks/crypto"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{server.URL + "/alpha", server.URL + "/beta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cards = %#v, want %#v", got, want)
	}
}
