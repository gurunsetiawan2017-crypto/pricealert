package grouping

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/pricealert/pricealert/internal/domain"
)

type normalizationCase struct {
	Name           string `json:"name"`
	Title          string `json:"title"`
	WantNormalized string `json:"want_normalized"`
}

type groupingCase struct {
	Name              string           `json:"name"`
	ScanJobID         string           `json:"scan_job_id"`
	Listings          []fixtureListing `json:"listings"`
	WantGroupCount    int              `json:"want_group_count"`
	WantListingCounts []int            `json:"want_listing_counts"`
	WantBestPrices    []int64          `json:"want_best_prices"`
}

type fixtureListing struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	SellerName string `json:"seller_name"`
	Price      int64  `json:"price"`
	IsPromo    bool   `json:"is_promo"`
	URL        string `json:"url"`
}

func TestNormalizeTitleFixtures(t *testing.T) {
	var cases []normalizationCase
	loadFixture(t, "normalization_cases.json", &cases)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := NormalizeTitle(tc.Title)
			if got != tc.WantNormalized {
				t.Fatalf("NormalizeTitle(%q) = %q, want %q", tc.Title, got, tc.WantNormalized)
			}
		})
	}
}

func TestGroupFixtures(t *testing.T) {
	var cases []groupingCase
	loadFixture(t, "grouping_cases.json", &cases)

	service := NewService()

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			rawListings := make([]domain.RawListing, 0, len(tc.Listings))
			for _, listing := range tc.Listings {
				rawListings = append(rawListings, domain.RawListing{
					ID:         listing.ID,
					Title:      listing.Title,
					SellerName: listing.SellerName,
					Price:      listing.Price,
					IsPromo:    listing.IsPromo,
					URL:        listing.URL,
				})
			}

			grouped := service.Group(tc.ScanJobID, rawListings)

			if len(grouped) != tc.WantGroupCount {
				t.Fatalf("group count = %d, want %d", len(grouped), tc.WantGroupCount)
			}

			gotListingCounts := make([]int, 0, len(grouped))
			gotBestPrices := make([]int64, 0, len(grouped))
			for _, groupedListing := range grouped {
				if groupedListing.ScanJobID != tc.ScanJobID {
					t.Fatalf("grouped listing scan_job_id = %q, want %q", groupedListing.ScanJobID, tc.ScanJobID)
				}
				if groupedListing.GroupKey == "" {
					t.Fatalf("grouped listing has empty group key")
				}

				gotListingCounts = append(gotListingCounts, groupedListing.ListingCount)
				gotBestPrices = append(gotBestPrices, groupedListing.BestPrice)
			}

			sort.Ints(gotListingCounts)
			sort.Ints(tc.WantListingCounts)
			if !reflect.DeepEqual(gotListingCounts, tc.WantListingCounts) {
				t.Fatalf("listing counts = %v, want %v", gotListingCounts, tc.WantListingCounts)
			}

			sort.Slice(gotBestPrices, func(i, j int) bool { return gotBestPrices[i] < gotBestPrices[j] })
			sort.Slice(tc.WantBestPrices, func(i, j int) bool { return tc.WantBestPrices[i] < tc.WantBestPrices[j] })
			if !reflect.DeepEqual(gotBestPrices, tc.WantBestPrices) {
				t.Fatalf("best prices = %v, want %v", gotBestPrices, tc.WantBestPrices)
			}
		})
	}
}

func TestExtractAttributes(t *testing.T) {
	attrs := ExtractAttributes("bimoli minyak goreng 2l refill isi 2")

	if attrs.SizeToken != "2l" {
		t.Fatalf("size token = %q, want %q", attrs.SizeToken, "2l")
	}
	if attrs.BundleToken != "isi2" {
		t.Fatalf("bundle token = %q, want %q", attrs.BundleToken, "isi2")
	}
	if attrs.PackagingToken != "refill" {
		t.Fatalf("packaging token = %q, want %q", attrs.PackagingToken, "refill")
	}
	if attrs.BrandToken != "bimoli" {
		t.Fatalf("brand token = %q, want %q", attrs.BrandToken, "bimoli")
	}
}

func loadFixture(t *testing.T, fileName string, target any) {
	t.Helper()

	path := filepath.Join("..", "..", "..", "testdata", "grouping", fileName)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fileName, err)
	}

	if err := json.Unmarshal(content, target); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", fileName, err)
	}
}
