package tui

import (
	"fmt"
	"strings"
)

// renderQuotaHearts renders 5 hearts that fade from red to grey based on quota percentage.
// Full hearts are red (#990000), empty hearts are grey (#535360), transitioning hearts blend.
func renderQuotaHearts(percent int) string {
	if percent < 0 {
		// Error state - grey hearts
		return "[#535360]♥♥♥♥♥[-]"
	}

	// Clamp to 0-100
	if percent > 100 {
		percent = 100
	}

	// Calculate full hearts and partial heart
	// Each heart = 20%, so 5 hearts = 100%
	fullHearts := percent / 20     // 0-5 full hearts
	partialPercent := percent % 20 // 0-19 for the transitioning heart

	heart := "♥"
	var result string

	// True color: red (#990000) to grey (#535360)
	// RGB: red = (153, 0, 0), grey = (83, 83, 96)

	for i := 0; i < 5; i++ {
		var color string
		if i < fullHearts {
			// Full red heart
			color = "#990000"
		} else if i == fullHearts && partialPercent > 0 {
			// Transitioning heart - blend from red to grey
			// partialPercent 19 = almost full (red), 1 = almost empty (grey)
			// Linear interpolation: color = red + (grey - red) * (20 - partial) / 20
			ratio := float64(20-partialPercent) / 20.0
			r := int(153.0 - (153.0-83.0)*ratio)
			g := int(0.0 + 83.0*ratio)
			b := int(0.0 + 96.0*ratio)
			color = fmt.Sprintf("#%02X%02X%02X", r, g, b)
		} else {
			// Empty grey heart
			color = "#535360"
		}
		result += fmt.Sprintf("[%s]%s[-]", color, heart)
	}

	return result
}

// wrapText wraps text at maxWidth and indents all lines with the given indent.
// It respects existing newlines and wraps at word boundaries.
func wrapText(text string, indent string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	// Account for indent in available width
	availableWidth := maxWidth - len(indent)
	if availableWidth < 20 {
		availableWidth = 20
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for lineIdx, line := range lines {
		if lineIdx > 0 {
			result.WriteString("\n")
		}

		// Handle empty lines
		if strings.TrimSpace(line) == "" {
			result.WriteString(indent)
			continue
		}

		// Wrap this line
		words := strings.Fields(line)
		if len(words) == 0 {
			result.WriteString(indent)
			continue
		}

		currentLine := indent
		currentLen := len(indent)

		for _, word := range words {
			wordLen := len(word)

			if currentLen+1+wordLen > maxWidth && currentLen > len(indent) {
				// Need to wrap - start new line
				result.WriteString(currentLine)
				result.WriteString("\n")
				currentLine = indent + word
				currentLen = len(indent) + wordLen
			} else {
				// Add word to current line
				if currentLen > len(indent) {
					currentLine += " "
					currentLen++
				}
				currentLine += word
				currentLen += wordLen
			}
		}

		// Write remaining content
		result.WriteString(currentLine)
	}

	return result.String()
}
