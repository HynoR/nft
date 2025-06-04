package main

import (
	"bufio"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

func (s *NatService) DownLoadFromRemote() {
	// 创建一个配置合理的HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	for _, path := range s.configPath {
		// 检查文件是否包含远程URL
		remoteURL := getRemoteURLFromFile(path)
		if remoteURL == "" {
			continue // 跳过没有远程URL的文件
		}

		// 下载并更新文件
		downloadAndUpdateFile(client, path, remoteURL)
	}
}

// 从文件中提取远程URL（如果存在）
func getRemoteURLFromFile(path string) string {
	// 只读取文件的第一行来检查远程URL标记
	file, err := os.Open(path)
	if err != nil {
		slog.Error("Failed to open config file", "path", path, "error", err)
		return ""
	}
	defer file.Close()

	// 使用Scanner仅读取第一行，避免加载整个文件
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return "" // 文件为空
	}

	firstLine := scanner.Text()
	if !strings.HasPrefix(firstLine, "# @Remote=") {
		return ""
	}

	remoteURL := strings.TrimPrefix(firstLine, "# @Remote=")
	remoteURL = strings.TrimSpace(remoteURL)

	// URL验证
	if !strings.HasPrefix(remoteURL, "http://") && !strings.HasPrefix(remoteURL, "https://") {
		slog.Error("Invalid remote URL scheme", "path", path, "url", remoteURL)
		return ""
	}

	return remoteURL
}

// 从远程URL下载并更新文件
func downloadAndUpdateFile(client *http.Client, path, remoteURL string) {
	slog.Info("Downloading from remote URL", "path", path, "url", remoteURL)

	// 添加简单的重试逻辑
	var resp *http.Response
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err = client.Get(remoteURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}

		if resp != nil {
			resp.Body.Close()
		}

		if attempt < maxRetries {
			slog.Warn("Retry downloading", "path", path, "url", remoteURL, "attempt", attempt, "error", err)
			time.Sleep(time.Duration(attempt) * time.Second) // 指数退避
		}
	}

	// 处理下载错误
	if err != nil {
		slog.Error("Failed to download after retries", "path", path, "url", remoteURL, "error", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		slog.Error("Failed to download from remote URL", "path", path, "url", remoteURL, "status", resp.Status)
		resp.Body.Close()
		return
	}
	defer resp.Body.Close()

	// 读取响应内容
	remoteContent, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read remote content", "path", path, "url", remoteURL, "error", err)
		return
	}

	// 追加同步时间
	remoteContent = append(remoteContent, []byte("\n# @Synced="+time.Now().Format(time.RFC3339)+"\n")...)

	// 直接写入原文件，避免替换文件导致的 inotify 失效
	if err := os.WriteFile(path, remoteContent, 0644); err != nil {
		slog.Error("Failed to write config file", "path", path, "error", err)
		return
	}

	slog.Info("Successfully downloaded and updated from remote URL", "path", path, "url", remoteURL)
}
