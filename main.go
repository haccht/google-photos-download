package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

var (
	secretFile = "client_secret.json"
	cacheToken = flag.Bool("cachetoken", true, "Cache the OAuth 2.0 token")
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] dirpath\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}

	dirPath := flag.Arg(0)

	service, err := getPhotosLibraryService()
	if err != nil {
		log.Fatalf("Unable to get photoslibrary: %v", err)
	}

	if err := downloadAllPhotos(service, dirPath); err != nil {
		log.Fatalf("Unable to download media item: %v", err)
	}
}
