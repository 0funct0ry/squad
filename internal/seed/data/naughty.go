// Package data holds small curated string pools for seed generators that
// aren't sensibly generated (naughty strings, domain-specific lookups).
package data

// NaughtyStrings is a curated subset of the public "big list of naughty
// strings" project - classic edge cases used to stress-test string handling.
var NaughtyStrings = []string{
	"",
	"   ",
	"'; DROP TABLE students;--",
	"' OR '1'='1",
	"<script>alert(1)</script>",
	"../../../../etc/passwd",
	"\x00",
	"NULL",
	"undefined",
	"NaN",
	"-1",
	"0",
	"\U0001D57F\U0001D5CA\U0001D598\U0001D5D9",
	"\U0001F600\U0001F389\U0001F4A5",
	"Ω≈ç√∫˜µ≤≥÷",
	"\t\n\r",
	"a very long string that repeats itself over and over and over and over and over and over and over and over and over again to stress test length handling",
	"1;DROP TABLE users",
	"%00",
	"admin'--",
}
