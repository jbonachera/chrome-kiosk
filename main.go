package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/chromedp/chromedp"
)

type ChromeInstance struct {
	ctx context.Context
}

type NavigateRequest struct {
	URL string `json:url"`
}

func (c *ChromeInstance) GoTo(url string) {
	log.Printf("going to %s", url)
	err := chromedp.Run(c.ctx, chromedp.Navigate(url))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("gone to %s", url)
}

func main() {
	quit := make(chan os.Signal)
	signal.Notify(quit,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("noerrdialogs", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.Flag("start-fullscreen", true),
		chromedp.Flag("mute-audio", false),
	)
	if os.Getenv("https_proxy") != "" {
		log.Printf("using proxy server")
		opts = append(opts, chromedp.ProxyServer(os.Getenv("https_proxy")))
	}

	done := make(chan struct{})
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)

	ctx, _ := chromedp.NewContext(allocCtx)
	go func() {
		defer close(done)
		<-ctx.Done()
	}()
	go func() {
		<-quit
		cancelAlloc()
	}()
	instance := &ChromeInstance{
		ctx: ctx,
	}
	instance.GoTo("https://logs-ng.ftntech.fr")
	mux := http.NewServeMux()
	mux.HandleFunc("/navigate/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "kiosk")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		payload := NavigateRequest{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprintf(`{"error": %q}`, err.Error())))
		}
		if payload.URL != "" {
			instance.GoTo(payload.URL)
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	go func() {
		http.ListenAndServe("127.0.0.1:8080", mux)
	}()
	<-done
}
