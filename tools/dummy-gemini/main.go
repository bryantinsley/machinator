package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	// Define flags
	_ = flag.String("prompt", "", "Prompt for the LLM")
	_ = flag.String("repo", "", "Repository path")
	_ = flag.Int("tokens", 0, "Token limit")
	_ = flag.String("model", "", "Model name")
	_ = flag.Float64("temperature", 0.0, "Sampling temperature")
	isJSON := flag.Bool("json", false, "Output in JSON format")

	flag.Parse()

	mode := os.Getenv("GEMINI_MODE")

	switch mode {
	case "ERROR":
		fmt.Fprintln(os.Stderr, "Quota exceeded")
		os.Exit(1)
	case "STUCK":
		time.Sleep(30 * time.Second)
		// After sleep, behave like HAPPY or just exit?
		// Usually STUCK means it hangs, but for a mock it should probably finish eventually or be killed.
		// The prompt says "simulating hang", so 30s is fine.
		printHappy(*isJSON)
	case "HAPPY":
		fallthrough
	default:
		printHappy(*isJSON)
	}
}

func printHappy(isJSON bool) {
	if isJSON {
		fmt.Println(`{"response": "mock response"}`)
	} else {
		fmt.Println("mock response")
	}
}
