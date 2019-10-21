package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
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

func download(item *photoslibrary.MediaItem, itemMap map[string]string, rootDirpath string) error {
	creationTime, err := time.Parse(time.RFC3339, item.MediaMetadata.CreationTime)
	if err != nil {
		return err
	}

	dirpath := path.Join(
		rootDirpath,
		fmt.Sprintf("%04d", creationTime.Year()),
		fmt.Sprintf("%02d", creationTime.Month()),
	)

	if err := os.MkdirAll(dirpath, 0755); err != nil {
		return err
	}

	filename := item.Filename
	filepath := path.Join(dirpath, filename)
	if id, ok := itemMap[filepath]; ok && id != item.Id {
		extname := path.Ext(filepath)
		filename = strings.TrimSuffix(filename, extname) + "-" + item.Id + extname
		filepath = path.Join(dirpath, filename)
	}

	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		log.Printf("Downloading \"%s\"\n", filepath)

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
		log.Printf("File already exists: \"%s\"\n", filepath)
	}

	itemMap[filepath] = item.Id
	return nil
}

func DownloadPhotos(service *photoslibrary.Service, dirpath string) error {
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

	itemMap := make(map[string]string)
	for {
		select {
		case item, ok := <-mediaChan:
			if !ok {
				break
			}

			err := download(item, itemMap, dirpath)
			if err != nil {
				log.Printf("Unable to download media item: %s\n", err)
			}
		case err := <-errorChan:
			return err
		}
	}

	return nil
}
