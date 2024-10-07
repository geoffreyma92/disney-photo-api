package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// APIResponse represents the top-level response structure
type APIResponse struct {
	Status  int    `json:"status"`
	Message string `json:"msg"`
	Result  Result `json:"result"`
	LocalIP int    `json:"localIp"`
}

// Result represents the result object in the response
type Result struct {
	Photos []Photo `json:"photos"`
	Time   int64   `json:"time"`
}

// Photo represents each photo in the response
type Photo struct {
	ID            string    `json:"_id"`
	IsFavorite    bool      `json:"isFavorite"`
	IsLike        bool      `json:"isLike"`
	ExpireDate    string    `json:"expireDate"`
	Watermarked   bool      `json:"watermarked"`
	EnImage       bool      `json:"enImage"`
	IsPaid        bool      `json:"isPaid"`
	ShootDate     string    `json:"shootDate"`
	StrShootOn    string    `json:"strShootOn"`
	PresetID      string    `json:"presetId"`
	SiteID        string    `json:"siteId"`
	PhotoCode     string    `json:"photoCode"`
	LocationID    string    `json:"locationId"`
	ShootOn       time.Time `json:"shootOn"`
	ExtractOn     time.Time `json:"extractOn"`
	Thumbnail     Thumbnail `json:"thumbnail"`
	ParentID      string    `json:"parentId"`
	ModifiedOn    time.Time `json:"modifiedOn"`
	MimeType      string    `json:"mimeType"`
	BundleWithPPP bool      `json:"bundleWithPPP"`
	CreatedBy     string    `json:"createdBy"`
	AllowDownload bool      `json:"allowDownload"`
	IsFree        bool      `json:"isFree"`
	Disabled      bool      `json:"disabled"`
	OriginalInfo  struct {
		Width        int      `json:"width"`
		Height       int      `json:"height"`
		URL          string   `json:"url"`
		EditHistorys []string `json:"editHistorys"`
	} `json:"originalInfo"`
	Comments      []interface{} `json:"comments"`
	LikeCount     int           `json:"likeCount"`
	EditCount     int           `json:"editCount"`
	ShareInfo     []interface{} `json:"shareInfo"`
	VisitedCount  int           `json:"visitedCount"`
	DownloadCount int           `json:"downloadCount"`
	CustomerIDs   []struct {
		Code    string   `json:"code"`
		CType   string   `json:"cType"`
		UserIDs []string `json:"userIds"`
	} `json:"customerIds"`
}

// Thumbnail represents the thumbnail structure
type Thumbnail struct {
	X1024 ThumbnailSize `json:"x1024"`
	X512  ThumbnailSize `json:"x512"`
	W512  ThumbnailSize `json:"w512"`
	X128  ThumbnailSize `json:"x128"`
}

// ThumbnailSize represents each size variant of a thumbnail
type ThumbnailSize struct {
	Path   string `json:"path"`
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

const (
	baseURL    = "https://www.disneyphotopass.com.hk/"
	outputDir  = "disney_photos" // The directory where photos will be saved
)

// PhotoDownloader handles concurrent downloads of photos
type PhotoDownloader struct {
	client *http.Client
	wg     sync.WaitGroup
}

func NewPhotoDownloader() *PhotoDownloader {
	return &PhotoDownloader{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (pd *PhotoDownloader) downloadPhoto(url, filepath string) error {
	resp, err := pd.client.Get(url)
	if err != nil {
		return fmt.Errorf("error downloading image: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (pd *PhotoDownloader) processPhoto(photo Photo, sizes []string) {
	pd.wg.Add(1)
	go func() {
		defer pd.wg.Done()

		for _, size := range sizes {
			var thumbnailURL string
			var sizeStr string

			switch size {
			case "x1024":
				thumbnailURL = photo.Thumbnail.X1024.URL
				sizeStr = "1024x"
			case "x128":
				thumbnailURL = photo.Thumbnail.X128.URL
				sizeStr = "128x"
			default:
				fmt.Printf("Unsupported size: %s\n", size)
				continue
			}

			if thumbnailURL == "" {
				fmt.Printf("No URL found for size %s in photo %s\n", size, photo.PhotoCode)
				continue
			}

			fullURL := baseURL + thumbnailURL
			filename := fmt.Sprintf("%s_%s.jpg", photo.PhotoCode, sizeStr)
			filepath := filepath.Join(outputDir, filename)

			fmt.Printf("Downloading %s...\n", filename)
			err := pd.downloadPhoto(fullURL, filepath)
			if err != nil {
				fmt.Printf("Error downloading %s: %v\n", filename, err)
			} else {
				fmt.Printf("Successfully downloaded %s\n", filename)
			}
		}
	}()
}

func getAPIResponse(apiURL string) (*APIResponse, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	var result APIResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	return &result, nil
}

func main() {
	// Create output directory
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}

	apiURL := "https://api.disneyphotopass.com.hk/shoppingapi/p/getPhotosByConditions?tokenId=c8cad990-83d3-11ef-bc1f-4f799151c3b9&currentPageIndex=1&limit=400&sortField=shootOn&order=-1"

	response, err := getAPIResponse(apiURL)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d photos to download\n", len(response.Result.Photos))

	downloader := NewPhotoDownloader()
	sizes := []string{"x1024", "x128"}

	for _, photo := range response.Result.Photos {
		downloader.processPhoto(photo, sizes)
	}

	// Wait for all downloads to complete
	downloader.wg.Wait()
	fmt.Println("All downloads completed!")
}