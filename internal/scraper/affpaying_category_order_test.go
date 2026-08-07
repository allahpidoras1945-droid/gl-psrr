package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestCategoryCrawlerFinishesOneCategoryBeforeNextAndContinuesPastGlobalDuplicates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		category := request.URL.Path
		page := request.URL.Query().Get("page")
		switch {
		case category == "/gambling" && page == "1":
			_, _ = writer.Write([]byte(`<a href="/shared">Shared</a>`))
		case category == "/gambling" && page == "2":
			_, _ = writer.Write([]byte(`<a href="/gambling-two">Gambling Two</a>`))
		case category == "/crypto" && page == "1":
			_, _ = writer.Write([]byte(`<a href="/shared">Shared Again</a>`))
		case category == "/crypto" && page == "2":
			_, _ = writer.Write([]byte(`<a href="/crypto-two">Crypto Two</a>`))
		}
	}))
	defer server.Close()

	crawler := NewCategoryCrawler(ScraperConfig{Timeout: time.Second})
	got, err := crawler.DiscoverCardURLs(context.Background(), []string{server.URL + "/gambling", server.URL + "/crypto"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{server.URL + "/shared", server.URL + "/gambling-two", server.URL + "/crypto-two"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cards = %#v, want %#v", got, want)
	}
}
