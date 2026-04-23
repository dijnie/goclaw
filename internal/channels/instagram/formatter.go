package instagram

// instagramMaxChars is the maximum message length for Instagram DM (characters, not bytes).
const instagramMaxChars = 1000

// FormatMessage truncates content to Instagram DM's rune limit.
// Uses rune-based truncation to preserve UTF-8 validity for non-ASCII content
// (Vietnamese, Chinese, emoji) — byte slicing can split a multibyte codepoint
// and produce invalid UTF-8 that the Graph API rejects.
func FormatMessage(content string) string {
	runes := []rune(content)
	if len(runes) <= instagramMaxChars {
		return content
	}
	return string(runes[:instagramMaxChars-3]) + "..."
}
