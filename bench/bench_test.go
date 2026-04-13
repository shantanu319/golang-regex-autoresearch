package bench

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	regexp "autoresearch-regex/regexp-opt"
)

var haystackCache sync.Map

func loadHaystack(name string) string {
	if text, ok := haystackCache.Load(name); ok {
		return text.(string)
	}

	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	text := string(data)
	haystackCache.Store(name, text)
	return text
}

func BenchmarkLiteralMatch(b *testing.B) {
	re := regexp.MustCompile("Sherlock Holmes")
	haystack := loadHaystack("sherlock.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.FindAllStringIndex(haystack, -1)
	}
}

func BenchmarkLiteralCaseInsensitive(b *testing.B) {
	re := regexp.MustCompile("(?i)sherlock holmes")
	haystack := loadHaystack("sherlock.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.FindAllStringIndex(haystack, -1)
	}
}

func BenchmarkAlternationMedium(b *testing.B) {
	re := regexp.MustCompile("Sherlock|Watson|Holmes|Moriarty|Lestrade")
	haystack := loadHaystack("sherlock.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.FindAllStringIndex(haystack, -1)
	}
}

func BenchmarkCharacterClass(b *testing.B) {
	re := regexp.MustCompile("[a-zA-Z]+@[a-zA-Z]+\\.[a-zA-Z]+")
	haystack := loadHaystack("mixed-content.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.FindAllStringIndex(haystack, -1)
	}
}

func BenchmarkUnicodeWordBoundary(b *testing.B) {
	re := regexp.MustCompile(`\b\w{4,}\b`)
	haystack := loadHaystack("sherlock.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.FindAllStringIndex(haystack, -1)
	}
}

func BenchmarkBoundedRepeat(b *testing.B) {
	re := regexp.MustCompile("[a-z]{2,4}ing")
	haystack := loadHaystack("sherlock.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.FindAllStringIndex(haystack, -1)
	}
}

func BenchmarkNonMatch(b *testing.B) {
	re := regexp.MustCompile("ZZZZZNOTFOUND")
	haystack := loadHaystack("sherlock.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.FindAllStringIndex(haystack, -1)
	}
}

func BenchmarkCompileSimple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		regexp.Compile("[a-z]+")
	}
}

func BenchmarkCompileComplex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		regexp.Compile(`(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)`)
	}
}
