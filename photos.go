package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	photoslibrary "github.com/nekr0z/gphotoslibrary"
	"google.golang.org/api/googleapi"
)

const (
	waitTime = 10 * time.Second
	pageSize = 100
)

func getPhotosLibraryService() (*photoslibrary.Service, error) {
	client, err := newOAuthClient(photoslibrary.PhotoslibraryReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("Unable to create api client: %v", err)
	}

	service, err := photoslibrary.New(client)
	if err != nil {
		return nil, fmt.Errorf("Unable to create photoslibrary service: %v", err)
	}

	return service, nil
}

func download(item *photoslibrary.MediaItem, dirPath string) error {
	creationTime, err := time.Parse(time.RFC3339, item.MediaMetadata.CreationTime)
	if err != nil {
		return err
	}

	filepath := path.Join(
		dirPath,
		fmt.Sprintf("%04d", creationTime.Year()),
		fmt.Sprintf("%02d", creationTime.Month()),
		item.Filename,
	)

	if err := os.MkdirAll(path.Dir(filepath), 0755); err != nil {
		return err
	}

	// TODO: Add feature to download files with same filenames and different IDs.
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		log.Printf("Downloading \"%s\"\n", item.Filename)

		mediaUrl := item.BaseUrl
		if item.MediaMetadata.Photo == nil {
			mediaUrl = mediaUrl + "=dv"
		} else {
			mediaUrl = mediaUrl + "=d"
		}

		file, err := os.Create(filepath)
		if err != nil {
			return err
		}
		defer file.Close()

		resp, err := http.Get(mediaUrl)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return err
		}
	} else {
		log.Printf("File already exists: \"%s\"\n", item.Filename)
	}

	return nil
}

func DownloadPhotos(service *photoslibrary.Service, dirPath string) error {
	mediaItemsService := photoslibrary.NewMediaItemsService(service)

	mediaChan := make(chan *photoslibrary.MediaItem, 100)
	errorChan := make(chan error, 1)
	pageToken := ""

	go func() {
		for {
			request := &photoslibrary.SearchMediaItemsRequest{PageSize: int64(pageSize), PageToken: pageToken}
			resp, err := mediaItemsService.Search(request).Do()
			if err != nil {
				if apiError, ok := err.(*googleapi.Error); ok {
					switch apiError.Code {
					case 429, 500, 502, 503:
						log.Printf("Unable to read media items: %s\n", err)
						continue
					}
				}
				errorChan <- err
			}

			for _, mediaItem := range resp.MediaItems {
				mediaChan <- mediaItem
			}

			pageToken = resp.NextPageToken
			if pageToken == "" {
				close(mediaChan)
			}
		}
	}()

	for {
		select {
		case item, ok := <-mediaChan:
			if !ok {
				break
			}

			err := download(item, dirPath)
			if err != nil {
				log.Printf("Unable to download media item: %s\n", err)
			}
		case err := <-errorChan:
			return err
		}
	}

	return nil
}
