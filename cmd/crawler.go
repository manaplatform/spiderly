package main

import (
	"flag"
	"log"
	"strings"

	"spiderly/internal/scraper"
)

func main() {
	urlFlag := flag.String("url", "", "Target URL or domain, example: example.com or https://example.com")
	flag.Parse()

	if *urlFlag == "" {
		log.Fatal("please provide a URL with -url")
	}

	targetURL := strings.TrimSpace(*urlFlag)

	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	scraper.ExtractData(targetURL)

	log.Println("Crawling completed!")
}
