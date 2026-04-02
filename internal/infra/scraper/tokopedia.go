package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pricealert/pricealert/internal/config"
	"github.com/pricealert/pricealert/internal/domain"
)

const tokopediaSource = "tokopedia"

// Validated against the local reference project findings dated March 26, 2026.
const tokopediaSearchQuery = `
query SearchProductV5Query($params:String!){
  searchProductV5(params:$params){
    header{
      responseCode
    }
    data{
      products{
        name
        url
        shop{
          name
        }
        price{
          text
          number
          original
          discountPercentage
        }
      }
    }
  }
}
`

type Tokopedia struct {
	endpoint   string
	rows       int
	httpClient *http.Client
}

type graphQLRequest struct {
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
	Query         string                 `json:"query"`
}

type searchEnvelope []struct {
	Data struct {
		SearchProductV5 struct {
			Header struct {
				ResponseCode int `json:"responseCode"`
			} `json:"header"`
			Data struct {
				Products []searchProduct `json:"products"`
			} `json:"data"`
		} `json:"searchProductV5"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type searchProduct struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Shop struct {
		Name string `json:"name"`
	} `json:"shop"`
	Price struct {
		Text               string `json:"text"`
		Number             int64  `json:"number"`
		Original           string `json:"original"`
		DiscountPercentage int    `json:"discountPercentage"`
	} `json:"price"`
}

func NewTokopedia(cfg config.ScraperConfig) *Tokopedia {
	return NewTokopediaWithHTTPClient(
		cfg,
		&http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second},
	)
}

func NewTokopediaWithHTTPClient(cfg config.ScraperConfig, httpClient *http.Client) *Tokopedia {
	return &Tokopedia{
		endpoint:   cfg.TokopediaSearchEndpoint,
		rows:       cfg.RowsPerScan,
		httpClient: httpClient,
	}
}

func (t *Tokopedia) FetchListings(ctx context.Context, keyword domain.TrackedKeyword) ([]domain.RawListing, error) {
	query := strings.TrimSpace(keyword.Keyword)
	if query == "" {
		return nil, fmt.Errorf("keyword is required")
	}

	payload, err := buildTokopediaSearchPayload(query, t.rows)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create tokopedia search request: %w", err)
	}

	setTokopediaHeaders(req, query)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tokopedia search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read tokopedia search response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected tokopedia search status: %d", resp.StatusCode)
	}

	return parseTokopediaSearchResponse(body)
}

func buildTokopediaSearchPayload(query string, rows int) ([]byte, error) {
	params := url.Values{}
	params.Set("device", "desktop")
	params.Set("ob", "23")
	params.Set("page", "1")
	params.Set("q", query)
	params.Set("rows", strconv.Itoa(rows))
	params.Set("safe_search", "false")
	params.Set("source", "search")
	params.Set("st", "product")
	params.Set("user_id", "0")
	params.Set("l_name", "sre")

	payload := []graphQLRequest{{
		OperationName: "SearchProductV5Query",
		Variables: map[string]interface{}{
			"params": params.Encode(),
		},
		Query: tokopediaSearchQuery,
	}}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal tokopedia search payload: %w", err)
	}

	return body, nil
}

func setTokopediaHeaders(req *http.Request, query string) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://www.tokopedia.com")
	req.Header.Set("Referer", "https://www.tokopedia.com/search?st=product&q="+url.QueryEscape(query))
}

func parseTokopediaSearchResponse(body []byte) ([]domain.RawListing, error) {
	var envelope searchEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse tokopedia search response: %w", err)
	}

	if len(envelope) == 0 {
		return nil, fmt.Errorf("tokopedia search response is empty")
	}

	if len(envelope[0].Errors) > 0 {
		return nil, fmt.Errorf("tokopedia graphql error: %s", envelope[0].Errors[0].Message)
	}

	products := envelope[0].Data.SearchProductV5.Data.Products
	listings := make([]domain.RawListing, 0, len(products))
	for _, product := range products {
		price := firstPositive(product.Price.Number, parsePriceString(product.Price.Text))
		if price <= 0 {
			continue
		}

		originalPrice := firstPositive(parsePriceString(product.Price.Original))
		listing := domain.RawListing{
			Source:     tokopediaSource,
			Title:      strings.TrimSpace(product.Name),
			SellerName: strings.TrimSpace(product.Shop.Name),
			Price:      price,
			IsPromo:    product.Price.DiscountPercentage > 0 || originalPrice > price,
			URL:        normalizeTokopediaURL(product.URL),
		}
		if originalPrice > 0 {
			listing.OriginalPrice = &originalPrice
		}

		listings = append(listings, listing)
	}

	return listings, nil
}

func normalizeTokopediaURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "https://www.tokopedia.com" + value
}

func parsePriceString(value string) int64 {
	if value == "" {
		return 0
	}

	filtered := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
	if filtered == "" {
		return 0
	}

	price, err := strconv.ParseInt(filtered, 10, 64)
	if err != nil {
		return 0
	}
	return price
}

func firstPositive(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
