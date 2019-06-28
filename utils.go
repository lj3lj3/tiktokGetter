package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

const userAgent string = "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1"

var rootPath string

func init() {
	executable, _ := os.Executable()
	rootPath = path.Dir(executable)
}

func getDigitFromFontString(fontStr string) int {
	fontStr = strings.ToUpper(fontStr)[3:7]
	switch fontStr {
	case "E602", "E60E", "E618":
		return 1
	case "E605", "E610", "E617":
		return 2
	case "E604", "E611", "E61A":
		return 3
	case "E606", "E60C", "E619":
		return 4
	case "E607", "E60F", "E61B":
		return 5
	case "E608", "E612", "E61F":
		return 6
	case "E60A", "E613", "E61C":
		return 7
	case "E60B", "E614", "E61D":
		return 8
	case "E609", "E615", "E61E":
		return 9
	case "E603", "E60D", "E616":
		return 0
	default:
		fmt.Printf("not a valid font string: %s", fontStr)
		return 0
	}
}

func mergeMaps(maps ...map[string]string) map[string]string {
	finalMap := make(map[string]string)
	for _, mapTmp := range maps {
		for key, value := range mapTmp {
			finalMap[key] = value // Overwrite
		}
	}
	return finalMap
}

func httpGet(url string) (*http.Response, error) {
	// ready to get url content
	var client = &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("error in creating requests: %v \n", err)
		return nil, err
	}
	// set up header, let server treat us as mobile browser
	req.Header.Set("User-Agent", userAgent)

	response, err := client.Do(req)
	if err != nil {
		fmt.Printf("error in getting response: %v \n", err)
		return nil, err
	}
	return response, nil
}

func downloadFile(url string, fileName string) (string, error) {
	// Create folders
	folder := rootPath + "/wwwroot/upload/getter/"
	if err := os.MkdirAll(folder, 0755); err != nil {
		fmt.Printf("error in making folders : %v \n", err)
		return "", err
	}

	// ready to get url content
	resp, err := httpGet(url)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Get extension from response
	extension := "jpg"
	contentTypes := strings.Split(resp.Header.Get("Content-Type"), "/")
	if len(contentTypes) > 1 {
		extension = contentTypes[1]
	}

	// Create file
	file := folder + fileName + "." + extension
	if _, err := os.Stat(file); os.IsNotExist(err) {
		// Not exists
		startTime := time.Now()
		fmt.Printf("downloading file: %s \n", fileName)

		filePtr, err := os.Create(file)
		if err != nil {
			fmt.Printf("error in creating file: %v \n", err)
			return "", err
		}
		defer func() {
			_ = filePtr.Close()
		}()

		if _, err := io.Copy(filePtr, resp.Body); err != nil {
			fmt.Printf("error in writring file: %v \n", err)
			return "", err
		}

		fmt.Printf("downloading file DONE: %s,%s, time cost:%d \n", fileName, url, time.Since(startTime))
	}

	return "/upload/getter/" + fileName + "." + extension, nil
}
