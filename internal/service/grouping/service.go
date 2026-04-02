package grouping

import (
	"sort"
	"strings"

	"github.com/pricealert/pricealert/internal/domain"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

type analyzedListing struct {
	raw             domain.RawListing
	normalizedTitle string
	tokens          []string
	attributes      Attributes
}

type groupCandidate struct {
	listings []analyzedListing
}

func (s *Service) Group(scanJobID string, rawListings []domain.RawListing) []domain.GroupedListing {
	if len(rawListings) == 0 {
		return []domain.GroupedListing{}
	}

	analyzed := make([]analyzedListing, 0, len(rawListings))
	for _, listing := range rawListings {
		normalized := NormalizeTitle(listing.Title)
		analyzed = append(analyzed, analyzedListing{
			raw:             listing,
			normalizedTitle: normalized,
			tokens:          canonicalTokens(normalized),
			attributes:      ExtractAttributes(normalized),
		})
	}

	sort.SliceStable(analyzed, func(i, j int) bool {
		if analyzed[i].raw.Price != analyzed[j].raw.Price {
			return analyzed[i].raw.Price < analyzed[j].raw.Price
		}

		if analyzed[i].normalizedTitle != analyzed[j].normalizedTitle {
			return analyzed[i].normalizedTitle < analyzed[j].normalizedTitle
		}

		return analyzed[i].raw.ID < analyzed[j].raw.ID
	})

	groups := make([]groupCandidate, 0)
	for _, listing := range analyzed {
		assigned := false

		for index := range groups {
			if groups[index].accepts(listing) {
				groups[index].listings = append(groups[index].listings, listing)
				assigned = true
				break
			}
		}

		if !assigned {
			groups = append(groups, groupCandidate{listings: []analyzedListing{listing}})
		}
	}

	result := make([]domain.GroupedListing, 0, len(groups))
	for _, group := range groups {
		result = append(result, group.toGroupedListing(scanJobID))
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].BestPrice != result[j].BestPrice {
			return result[i].BestPrice < result[j].BestPrice
		}

		return result[i].GroupKey < result[j].GroupKey
	})

	return result
}

func (g groupCandidate) accepts(candidate analyzedListing) bool {
	reference := g.representative()

	if hardMismatch(reference.attributes, candidate.attributes) {
		return false
	}

	if !priceCompatible(reference.raw.Price, candidate.raw.Price) {
		return false
	}

	return jaccardSimilarity(reference.tokens, candidate.tokens) >= defaultSimilarityThreshold
}

func (g groupCandidate) representative() analyzedListing {
	best := g.listings[0]

	for _, listing := range g.listings[1:] {
		if listing.raw.Price < best.raw.Price {
			best = listing
			continue
		}

		if listing.raw.Price == best.raw.Price && listing.raw.IsPromo && !best.raw.IsPromo {
			best = listing
			continue
		}

		if listing.raw.Price == best.raw.Price && listing.raw.IsPromo == best.raw.IsPromo && listing.normalizedTitle < best.normalizedTitle {
			best = listing
		}
	}

	return best
}

func (g groupCandidate) toGroupedListing(scanJobID string) domain.GroupedListing {
	representative := g.representative()
	return domain.GroupedListing{
		ID:                   "",
		ScanJobID:            scanJobID,
		GroupKey:             buildGroupKey(representative.tokens, representative.attributes),
		RepresentativeTitle:  representative.raw.Title,
		RepresentativeSeller: representative.raw.SellerName,
		BestPrice:            representative.raw.Price,
		OriginalPrice:        representative.raw.OriginalPrice,
		IsPromo:              representative.raw.IsPromo,
		ListingCount:         len(g.listings),
		SampleURL:            representative.raw.URL,
	}
}

func buildGroupKey(tokens []string, attrs Attributes) string {
	parts := make([]string, 0, len(tokens)+4)
	parts = append(parts, tokens...)

	if attrs.BrandToken != "" {
		parts = append(parts, "brand-"+attrs.BrandToken)
	}

	if attrs.SizeToken != "" {
		parts = append(parts, "size-"+attrs.SizeToken)
	}

	if attrs.BundleToken != "" {
		parts = append(parts, "bundle-"+attrs.BundleToken)
	}

	if attrs.PackagingToken != "" {
		parts = append(parts, "packaging-"+attrs.PackagingToken)
	}

	return strings.Join(parts, "-")
}
