package install_go

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"zenbox/print"

	"gopkg.in/cheggaaa/pb.v1"
)

var (
	// 这个 URL 国内可能访问不到,谁能提供反代吗?

	DefaultDownloadURLPrefix = "https://dl.google.com/go"

	// 上面地址不能访问将切换到下面地址
	StandbyDownloadURLPrefix = "http://216.58.200.240/golang"
	StandbyDownloadHost      = "storage.googleapis.com"

	DefaultProxyURL = ""
)

// https://dl.google.com/go
func downloadGolang(target string) (string, error) {
	if DefaultProxyURL != "" {
		os.Setenv("HTTPS_PROXY", DefaultProxyURL)
	}

	uri := fmt.Sprintf("%s/%s", DefaultDownloadURLPrefix, target)

	urlState := pingUrl(DefaultDownloadURLPrefix)
	if !urlState {
		uri = fmt.Sprintf("%s/%s", StandbyDownloadURLPrefix, target)
	}

	print.IF("开始下载 Go 安装包: %s\n", uri)

	req, err := http.NewRequest("GET", uri, nil)
	if !urlState {
		req.Host = StandbyDownloadHost
	}

	if err != nil {
		return "", err
	}
	req.Header.Add("User-Agent", fmt.Sprintf("golang.org-getgo/%s", target))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载 Go 安装包失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		return "", fmt.Errorf("下载 Go 安装包失败: HTTP %d: %s", resp.StatusCode, uri)
	}

	size, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		return "", err
	}

	cachePath := filepath.Join("cache", "downloads")
	os.MkdirAll(cachePath, os.ModePerm)
	targetName := filepath.Join(cachePath, target)
	os.Remove(targetName)
	targetFile, err := os.OpenFile(targetName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return "", err
	}
	defer targetFile.Close()

	bar := pb.New(size).SetUnits(pb.U_BYTES)
	bar.Start()

	h := sha256.New()
	w := io.MultiWriter(targetFile, h, bar)
	if _, err := io.Copy(w, resp.Body); err != nil {
		bar.Finish()
		return "", err
	}

	bar.Finish()

	req, err = http.NewRequest("GET", uri+".sha256", nil)
	if !urlState {
		req.Host = StandbyDownloadHost
	}
	if err != nil {
		return "", err
	}
	req.Header.Add("User-Agent", fmt.Sprintf("golang.org-getgo/%s", target))

	sresp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取文件 %s 失败: %v", uri, err)
	}
	defer sresp.Body.Close()

	if sresp.StatusCode > 299 {
		return "", fmt.Errorf("获取 %s.sha256 失败: %d", uri, sresp.StatusCode)
	}

	shasum, err := ioutil.ReadAll(sresp.Body)
	if err != nil {
		return "", err
	}

	sum := fmt.Sprintf("%x", h.Sum(nil))
	if sum != string(shasum) {
		return "", fmt.Errorf("下载的文件 HASH 与服务器的文件 HASH 不匹配: %s != %s", sum, string(shasum))
	}

	if err = ioutil.WriteFile(targetName+".sha256", shasum, 0600); err != nil {
		os.Remove(targetName)
		return "", err
	}

	return targetFile.Name(), nil
}

func pingUrl(url string) bool {
	// 超时3秒即代表地址无法访问
	timeout := time.Duration(3 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	resp, err := client.Head(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return true
}
