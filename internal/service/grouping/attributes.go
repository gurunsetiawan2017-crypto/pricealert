package grouping

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	sizePattern       = regexp.MustCompile(`\b\d+(?:ml|l|kg|g)\b`)
	bundleXPattern    = regexp.MustCompile(`\b\d+x\d+(?:ml|l|kg|g)\b`)
	bundlePackPattern = regexp.MustCompile(`\b(?:isi|pack|pak|pcs?|pc)\s*\d+\b`)
	bundleUnitPattern = regexp.MustCompile(`\b\d+\s*(?:pcs?|pc)\b`)
)

var packagingAliases = map[string]string{
	"refill": "refill",
	"botol":  "botol",
	"bottle": "botol",
	"pouch":  "pouch",
}

var knownBrands = map[string]struct{}{
	"bimoli":   {},
	"filma":    {},
	"tropical": {},
}

type Attributes struct {
	SizeToken      string
	BundleToken    string
	PackagingToken string
	BrandToken     string
}

func ExtractAttributes(normalizedTitle string) Attributes {
	tokens := strings.Fields(normalizedTitle)

	return Attributes{
		SizeToken:      extractSizeToken(tokens),
		BundleToken:    extractBundleToken(normalizedTitle, tokens),
		PackagingToken: extractPackagingToken(tokens),
		BrandToken:     extractBrandToken(tokens),
	}
}

func extractSizeToken(tokens []string) string {
	for _, token := range tokens {
		if sizePattern.MatchString(token) {
			return token
		}
	}

	return ""
}

func extractBundleToken(normalizedTitle string, tokens []string) string {
	if match := bundleXPattern.FindString(normalizedTitle); match != "" {
		return compactBundle(match)
	}

	for _, token := range tokens {
		if bundleUnitPattern.MatchString(token) {
			return compactBundle(token)
		}
	}

	if match := bundlePackPattern.FindString(normalizedTitle); match != "" {
		return compactBundle(match)
	}

	return ""
}

func extractPackagingToken(tokens []string) string {
	for _, token := range tokens {
		if normalized, ok := packagingAliases[token]; ok {
			return normalized
		}
	}

	return ""
}

func extractBrandToken(tokens []string) string {
	for _, token := range tokens {
		if _, ok := knownBrands[token]; ok {
			return token
		}
	}

	return ""
}

func compactBundle(bundle string) string {
	return strings.ReplaceAll(bundle, " ", "")
}

func canonicalTokens(normalizedTitle string) []string {
	tokens := strings.Fields(normalizedTitle)
	seen := make(map[string]struct{}, len(tokens))
	canonical := make([]string, 0, len(tokens))

	for _, token := range tokens {
		if token == "" {
			continue
		}

		if _, ok := seen[token]; ok {
			continue
		}

		seen[token] = struct{}{}
		canonical = append(canonical, token)
	}

	sort.Strings(canonical)
	return canonical
}

func bundleLooksSingle(attrs Attributes) bool {
	return attrs.BundleToken == ""
}

func normalizeBundleCount(bundle string) int {
	if bundle == "" {
		return 0
	}

	for _, token := range strings.Fields(strings.NewReplacer("x", " ", "isi", " ", "pack", " ", "pak", " ", "pcs", " ", "pc", " ").Replace(bundle)) {
		value, err := strconv.Atoi(token)
		if err == nil {
			return value
		}
	}

	return 0
}
