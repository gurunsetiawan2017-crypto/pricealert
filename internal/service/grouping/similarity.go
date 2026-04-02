package grouping

import "math"

const (
	defaultSimilarityThreshold = 0.75
	maxPriceRatio              = 2.0
)

func hardMismatch(left, right Attributes) bool {
	if left.SizeToken != "" && right.SizeToken != "" && left.SizeToken != right.SizeToken {
		return true
	}

	if left.BundleToken != "" || right.BundleToken != "" {
		if bundleLooksSingle(left) != bundleLooksSingle(right) {
			return true
		}

		if left.BundleToken != "" && right.BundleToken != "" && normalizeBundleCount(left.BundleToken) != normalizeBundleCount(right.BundleToken) {
			return true
		}
	}

	if left.BrandToken != "" && right.BrandToken != "" && left.BrandToken != right.BrandToken {
		return true
	}

	if left.PackagingToken != "" && right.PackagingToken != "" && left.PackagingToken != right.PackagingToken {
		return true
	}

	return false
}

func jaccardSimilarity(left, right []string) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}

	leftSet := make(map[string]struct{}, len(left))
	for _, token := range left {
		leftSet[token] = struct{}{}
	}

	intersection := 0
	unionSet := make(map[string]struct{}, len(left)+len(right))

	for _, token := range left {
		unionSet[token] = struct{}{}
	}

	for _, token := range right {
		if _, ok := leftSet[token]; ok {
			intersection++
		}

		unionSet[token] = struct{}{}
	}

	return float64(intersection) / float64(len(unionSet))
}

func priceCompatible(left, right int64) bool {
	if left <= 0 || right <= 0 {
		return false
	}

	high := math.Max(float64(left), float64(right))
	low := math.Min(float64(left), float64(right))
	return high/low <= maxPriceRatio
}
