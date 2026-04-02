package grouping

import (
	"regexp"
	"strings"
)

var (
	whitespacePattern = regexp.MustCompile(`\s+`)
	nonWordPattern    = regexp.MustCompile(`[^\p{L}\p{N}]+`)
	literPattern      = regexp.MustCompile(`\b(\d+)\s*(liter|ltr|lt)\b`)
	milliliterPattern = regexp.MustCompile(`\b(\d+)\s*ml\b`)
	kilogramPattern   = regexp.MustCompile(`\b(\d+)\s*kg\b`)
	gramPattern       = regexp.MustCompile(`\b(\d+)\s*gr\b`)
)

var safeStopPhrases = []string{
	"ready stock",
	"stok tersedia",
	"gratis ongkir",
	"best seller",
}

var safeStopTokens = map[string]struct{}{
	"promo":    {},
	"murah":    {},
	"diskon":   {},
	"original": {},
	"ori":      {},
	"termurah": {},
	"official": {},
}

func NormalizeTitle(title string) string {
	normalized := strings.ToLower(strings.TrimSpace(title))
	normalized = normalizeUnits(normalized)
	normalized = nonWordPattern.ReplaceAllString(normalized, " ")
	normalized = whitespacePattern.ReplaceAllString(normalized, " ")

	for _, phrase := range safeStopPhrases {
		normalized = strings.ReplaceAll(normalized, phrase, " ")
	}

	tokens := strings.Fields(normalized)
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, ok := safeStopTokens[token]; ok {
			continue
		}

		filtered = append(filtered, token)
	}

	return strings.Join(filtered, " ")
}

func normalizeUnits(value string) string {
	value = literPattern.ReplaceAllString(value, `${1}l`)
	value = milliliterPattern.ReplaceAllString(value, `${1}ml`)
	value = kilogramPattern.ReplaceAllString(value, `${1}kg`)
	value = gramPattern.ReplaceAllString(value, `${1}g`)
	return value
}
