package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/google/go-github/v45/github"
)

type PushHandler func(ctx context.Context, event *github.PushEvent) error

type WebhookServer struct {
	pushHandler PushHandler
	cancelFunc  context.CancelFunc
	mu          sync.Mutex
}

func NewWebhookServer(pushHandler PushHandler) *WebhookServer {
	return &WebhookServer{
		pushHandler: pushHandler,
	}
}

func (s *WebhookServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	const webhookURL = "/webhook"
	if request.RequestURI != webhookURL {
		http.Error(writer, "Not found", http.StatusNotFound)
		return
	}
	payload, err := io.ReadAll(request.Body)
	if err != nil {
		fmt.Println("failed to read payload:", err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer request.Body.Close()

	wt := github.WebHookType(request)
	event, err := github.ParseWebHook(wt, payload)
	if err != nil {
		fmt.Printf("failed to parse webhook(type: %s): %s\n", wt, err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}
	e, ok := event.(*github.PushEvent)
	if !ok {
		fmt.Println("unsupported event type:", wt)
		http.Error(writer, "Not found", http.StatusNotFound)
		return
	}
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.mu.Lock()
		if s.cancelFunc != nil {
			s.cancelFunc()
		}
		s.cancelFunc = cancel
		s.mu.Unlock()

		if err := s.pushHandler(ctx, e); err != nil {
			fmt.Println("failed to run push handler:", err)
			http.Error(writer, "Internal server error", http.StatusInternalServerError)
			return
		}
	}()

	writer.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(writer, "OK")

}
