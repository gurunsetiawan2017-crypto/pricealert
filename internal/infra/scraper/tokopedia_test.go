package scraper

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"net/http"
	"net/http/httptest"

	"github.com/pricealert/pricealert/internal/config"
	"github.com/pricealert/pricealert/internal/domain"
)

func TestParseTokopediaSearchResponse(t *testing.T) {
	body := mustReadFixture(t, "tokopedia_search_response.json")

	listings, err := parseTokopediaSearchResponse(body)
	if err != nil {
		t.Fatalf("parseTokopediaSearchResponse() error = %v", err)
	}
	if len(listings) != 2 {
		t.Fatalf("listings length = %d, want %d", len(listings), 2)
	}

	first := listings[0]
	if first.Source != "tokopedia" {
		t.Fatalf("first source = %q, want tokopedia", first.Source)
	}
	if first.Title != "Mouse Genius Wireless NX 7005" {
		t.Fatalf("first title = %q", first.Title)
	}
	if first.SellerName != "Indonesia Genius" {
		t.Fatalf("first seller = %q", first.SellerName)
	}
	if first.Price != 118990 {
		t.Fatalf("first price = %d", first.Price)
	}
	if first.OriginalPrice == nil || *first.OriginalPrice != 130000 {
		t.Fatalf("first original price = %v", first.OriginalPrice)
	}
	if !first.IsPromo {
		t.Fatalf("first promo = false, want true")
	}

	second := listings[1]
	if second.URL != "https://www.tokopedia.com/sabang-computer/mouse-genius-wireless-nx-7005" {
		t.Fatalf("second URL = %q", second.URL)
	}
	if second.OriginalPrice != nil {
		t.Fatalf("second original price = %v, want nil", second.OriginalPrice)
	}
}

func TestTokopediaFetchListingsBuildsSearchRequest(t *testing.T) {
	fixture := mustReadFixture(t, "tokopedia_search_response.json")
	var received []graphQLRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Origin"); got != "https://www.tokopedia.com" {
			t.Fatalf("Origin = %q", got)
		}
		if !strings.Contains(r.Header.Get("Referer"), "minyak+goreng+2L") {
			t.Fatalf("Referer = %q", r.Header.Get("Referer"))
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewTokopediaWithHTTPClient(config.ScraperConfig{
		TokopediaSearchEndpoint: server.URL,
		TimeoutSeconds:          5,
		RowsPerScan:             7,
	}, server.Client())

	listings, err := client.FetchListings(context.Background(), domain.TrackedKeyword{
		ID:      "kw_1",
		Keyword: "minyak goreng 2L",
	})
	if err != nil {
		t.Fatalf("FetchListings() error = %v", err)
	}
	if len(listings) != 2 {
		t.Fatalf("listings length = %d, want %d", len(listings), 2)
	}
	if len(received) != 1 {
		t.Fatalf("requests length = %d, want 1", len(received))
	}

	req := received[0]
	if req.OperationName != "SearchProductV5Query" {
		t.Fatalf("operation = %q", req.OperationName)
	}
	params, err := url.ParseQuery(asString(req.Variables["params"]))
	if err != nil {
		t.Fatalf("parse params: %v", err)
	}
	if params.Get("q") != "minyak goreng 2L" {
		t.Fatalf("q = %q", params.Get("q"))
	}
	if params.Get("rows") != "7" {
		t.Fatalf("rows = %q", params.Get("rows"))
	}
}

func TestParseTokopediaSearchResponseAllowsEmptyProducts(t *testing.T) {
	listings, err := parseTokopediaSearchResponse([]byte(`[{"data":{"searchProductV5":{"header":{"responseCode":0},"data":{"products":[]}}}}]`))
	if err != nil {
		t.Fatalf("parseTokopediaSearchResponse() error = %v", err)
	}
	if len(listings) != 0 {
		t.Fatalf("listings length = %d, want 0", len(listings))
	}
}

func TestTokopediaFetchListingsRetriesTransientHTTPStatus(t *testing.T) {
	fixture := mustReadFixture(t, "tokopedia_search_response.json")
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt < 3 {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewTokopediaWithHTTPClient(config.ScraperConfig{
		TokopediaSearchEndpoint: server.URL,
		TimeoutSeconds:          5,
		RowsPerScan:             10,
		RetryAttempts:           3,
		RetryBackoffMillis:      1,
	}, server.Client())

	listings, err := client.FetchListings(context.Background(), domain.TrackedKeyword{Keyword: "mouse"})
	if err != nil {
		t.Fatalf("FetchListings() error = %v", err)
	}
	if len(listings) != 2 {
		t.Fatalf("listings length = %d, want 2", len(listings))
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
}

func TestTokopediaFetchListingsDoesNotRetryPermanentHTTPStatus(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewTokopediaWithHTTPClient(config.ScraperConfig{
		TokopediaSearchEndpoint: server.URL,
		TimeoutSeconds:          5,
		RowsPerScan:             10,
		RetryAttempts:           3,
		RetryBackoffMillis:      1,
	}, server.Client())

	_, err := client.FetchListings(context.Background(), domain.TrackedKeyword{Keyword: "mouse"})
	if err == nil {
		t.Fatalf("FetchListings() error = nil, want error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want 1", attempts.Load())
	}
}

func TestParseTokopediaSearchResponseUsesFirstUsableEnvelope(t *testing.T) {
	body := []byte(`[
		{"data":{"searchProductV5":{"header":{"responseCode":0},"data":{}}}},
		{"data":{"searchProductV5":{"header":{"responseCode":0},"data":{"products":[
			{"name":"Produk A","url":"/shop/produk-a","shop":{"name":"Toko A"},"price":{"text":"Rp12.000","number":12000,"original":"","discountPercentage":0}}
		]}}}}
	]`)

	listings, err := parseTokopediaSearchResponse(body)
	if err != nil {
		t.Fatalf("parseTokopediaSearchResponse() error = %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("listings length = %d, want 1", len(listings))
	}
	if listings[0].Title != "Produk A" {
		t.Fatalf("title = %q, want Produk A", listings[0].Title)
	}
}

func TestParseTokopediaSearchResponseSkipsMalformedProducts(t *testing.T) {
	body := []byte(`[
		{"data":{"searchProductV5":{"header":{"responseCode":0},"data":{"products":[
			{"name":"Tanpa Harga","url":"/shop/tanpa-harga","shop":{"name":"Toko A"},"price":{"text":"","number":0,"original":"","discountPercentage":0}},
			{"name":"Tanpa URL","url":"","shop":{"name":"Toko B"},"price":{"text":"Rp10.000","number":10000,"original":"","discountPercentage":0}},
			{"name":"Valid","url":"/shop/valid","shop":{"name":"Toko C"},"price":{"text":"Rp15.000","number":15000,"original":"","discountPercentage":0}}
		]}}}}
	]`)

	listings, err := parseTokopediaSearchResponse(body)
	if err != nil {
		t.Fatalf("parseTokopediaSearchResponse() error = %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("listings length = %d, want 1", len(listings))
	}
	if listings[0].Title != "Valid" {
		t.Fatalf("title = %q, want Valid", listings[0].Title)
	}
}

func TestParseTokopediaSearchResponseSupportsSingleEnvelopeShape(t *testing.T) {
	body := []byte(`{
		"data":{
			"searchProductV5":{
				"header":{"responseCode":0},
				"data":{"products":[
					{"name":"Produk Tunggal","url":"/shop/produk-tunggal","shop":{"name":"Toko Tunggal"},"price":{"text":"Rp9.000","number":9000,"original":"","discountPercentage":0}}
				]}
			}
		}
	}`)

	listings, err := parseTokopediaSearchResponse(body)
	if err != nil {
		t.Fatalf("parseTokopediaSearchResponse() error = %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("listings length = %d, want 1", len(listings))
	}
	if listings[0].URL != "https://www.tokopedia.com/shop/produk-tunggal" {
		t.Fatalf("url = %q", listings[0].URL)
	}
}

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("testdata", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	return body
}

func asString(value interface{}) string {
	text, _ := value.(string)
	return text
}
