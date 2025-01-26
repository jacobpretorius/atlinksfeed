package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type ATPost struct {
	Did    string `json:"did"`
	TimeUs int64  `json:"time_us"`
	Type   string `json:"type"`
	Kind   string `json:"kind"`
	Commit struct {
		Rev        string `json:"rev"`
		Type       string `json:"type"`
		Operation  string `json:"operation"`
		Collection string `json:"collection"`
		Rkey       string `json:"rkey"`
		Record     struct {
			Type      string    `json:"$type"`
			CreatedAt time.Time `json:"createdAt"`
			Embed     struct {
				Type     string `json:"$type"`
				External struct {
					Description string `json:"description"`
					Title       string `json:"title"`
					URI         string `json:"uri"`
				} `json:"external"`
			} `json:"embed"`
			Facets []struct {
				Features []struct {
					Type string `json:"$type", omitempty`
					URI  string `json:"uri", omitempty`
				} `json:"features"`
				Index struct {
					ByteEnd   int `json:"byteEnd"`
					ByteStart int `json:"byteStart"`
				} `json:"index"`
			} `json:"facets"`
			Langs []string `json:"langs", omitempty`
			Text  string   `json:"text"`
		} `json:"record"`
		Cid string `json:"cid"`
	} `json:"commit"`
}

type WebSocketClient struct {
	url        string
	conn       *websocket.Conn
	done       chan struct{}
	maxRetries int
}

func NewWebSocketClient(url string) *WebSocketClient {
	return &WebSocketClient{
		url:        url,
		done:       make(chan struct{}),
		maxRetries: 5,
	}
}

func (c *WebSocketClient) Connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, c.url, nil)
	if err != nil {
		return err
	}

	// Configure connection
	conn.SetReadLimit(512 * 1024) // 512KB
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	c.conn = conn
	return nil
}

func (c *WebSocketClient) Close() {
	if c.conn != nil {
		c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.conn.Close()
	}
	close(c.done)
}

func ExtractUri(p ATPost) string {
	var uri string
	for _, facet := range p.Commit.Record.Facets {
		for _, feature := range facet.Features {
			if feature.Type == "app.bsky.richtext.facet#link" {
				uri = feature.URI
			}
		}
	}
	return uri
}

func (c *WebSocketClient) Listen(ctx context.Context) error {
	// Setup clean shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-shutdown:
			log.Println("Shutting down gracefully...")
			c.Close()
		case <-ctx.Done():
			log.Println("Context cancelled, shutting down...")
			c.Close()
		}
	}()

	retries := 0
	backoff := time.Second

	for {
		err := c.Connect(ctx)
		if err != nil {
			if retries >= c.maxRetries {
				return err
			}
			log.Printf("Connection failed, retrying in %v: %v", backoff, err)
			select {
			case <-time.After(backoff):
				retries++
				backoff *= 2 // exponential backoff
				continue
			case <-ctx.Done():
				return ctx.Err()
			case <-c.done:
				return nil
			}
		}

		// Reset retry count on successful connection
		retries = 0
		backoff = time.Second

		// Start reading messages
		for {
			var post ATPost
			err := c.conn.ReadJSON(&post)
			if err != nil {
				log.Printf("Read error: %v", err)
				c.conn.Close()
				break // Break inner loop to trigger reconnect
			}

			var uri = ExtractUri(post)
			if uri != "" {
				log.Printf("URI: %s", uri)
			}
		}

	}
}

func main() {
	log.Println("Starting feed listener... (Press Ctrl+C to stop)")

	client := NewWebSocketClient("wss://jetstream2.us-west.bsky.network/subscribe?wantedCollections=app.bsky.feed.post")

	// Setup context with cancellation on interrupt
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := client.Listen(ctx); err != nil {
		log.Printf("Fatal error: %v", err)
		os.Exit(1)
	}
}
