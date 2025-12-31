package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"golang.org/x/net/proxy"
)

func main() {
	fmt.Println("[INFO] Tor Scraper baslatildi")

	targetFile, err := os.Open("targets.yaml")
	if err != nil {
		fmt.Println("[ERR] targets.yaml bulunamadi:", err)
		return
	}
	defer targetFile.Close()

	httpClient, err := createTorHTTPClient()
	if err != nil {
		fmt.Println("[ERR] Tor proxy baglantisi kurulamadı:", err)
		return
	}

	checkTor(httpClient)

	logFile, err := os.Create("scan_report.log")
	if err != nil {
		fmt.Println("[ERR] Log dosyasi olusturulamadi:", err)
		return
	}
	defer logFile.Close()

	scanner := bufio.NewScanner(targetFile)
	index := 1

	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url == "" || strings.HasPrefix(url, "#") {
			continue
		}

		fmt.Printf("[INFO] Scanning (%d): %s\n", index, url)
		logFile.WriteString(fmt.Sprintf("[INFO] Scanning: %s -> ", url))

		if err := processTarget(httpClient, url, index); err != nil {
			fmt.Println("[ERR] FAILED:", err)
			logFile.WriteString("FAILED\n")
		} else {
			fmt.Println("[OK] SUCCESS")
			logFile.WriteString("SUCCESS\n")
		}

		fmt.Println("-----------------------------------")
		index++
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("[ERR] Dosya okuma hatasi:", err)
	}

	fmt.Println("[INFO] Tarama tamamlandi, rapor hazir")
}

func createTorHTTPClient() (*http.Client, error) {
	dialer, err := proxy.SOCKS5(
		"tcp",
		"127.0.0.1:9150",
		nil,
		proxy.Direct,
	)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Dial: dialer.Dial,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   40 * time.Second,
	}, nil
}

func checkTor(client *http.Client) {
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		fmt.Println("[WARN] Tor IP kontrolu yapilamadi:", err)
		return
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[WARN] IP okunamadi")
		return
	}

	fmt.Printf("[INFO] Tor cikis IP adresi: %s\n", strings.TrimSpace(string(ip)))
}

func processTarget(client *http.Client, url string, index int) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status %d", resp.StatusCode)
	}

	htmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	htmlFile := fmt.Sprintf("data_%d.html", index)
	if err := os.WriteFile(htmlFile, htmlData, 0644); err != nil {
		return err
	}

	return takeScreenshot(url, index)
}

func takeScreenshot(url string, index int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ProxyServer("socks5://127.0.0.1:9150"),
		chromedp.Flag("headless", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx)
	defer chromeCancel()

	var image []byte
	err := chromedp.Run(
		chromeCtx,
		chromedp.Navigate(url),
		chromedp.Sleep(6*time.Second),
		chromedp.FullScreenshot(&image, 85),
	)
	if err != nil {
		return err
	}

	fileName := fmt.Sprintf("view_%d.png", index)
	return os.WriteFile(fileName, image, 0644)
}
