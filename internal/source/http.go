package source

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/zaigie/palworld-server-tool/internal/logger"
	"github.com/zaigie/palworld-server-tool/internal/system"
)

var downloadClient = &http.Client{Timeout: 10 * time.Minute}

func DownloadFromHttp(url, way string) (string, error) {
	logger.Infof("downloading sav.zip from %s\n", url)
	resp, err := downloadClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("download failed with HTTP status %d", resp.StatusCode)
	}

	uuid := uuid.New().String()
	tempPath := filepath.Join(os.TempDir(), "palworldsav-http-"+way+"-"+uuid)
	absPath, err := filepath.Abs(tempPath)
	if err != nil {
		return "", err
	}

	if err = system.CleanAndCreateDir(absPath); err != nil {
		return "", err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = os.RemoveAll(absPath)
		}
	}()

	tempZipFilePath := filepath.Join(absPath, "sav.zip")
	defer os.Remove(tempZipFilePath)

	zipOut, err := os.Create(tempZipFilePath)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(zipOut, resp.Body)
	if err != nil {
		zipOut.Close()
		return "", err
	}
	if err := zipOut.Close(); err != nil {
		return "", err
	}

	err = system.UnzipDir(tempZipFilePath, absPath)
	if err != nil {
		return "", err
	}
	levelFilePath := filepath.Join(absPath, "Level.sav")
	succeeded = true
	logger.Info("sav.zip downloaded and extracted\n")
	return levelFilePath, nil
}
