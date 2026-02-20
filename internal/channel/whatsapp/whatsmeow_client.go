package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	// Register ncruces sqlite3 as a database/sql driver for whatsmeow's sqlstore.
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// Compile-time check that WhatsmeowClient satisfies WAClient.
var _ WAClient = (*WhatsmeowClient)(nil)

// WhatsmeowClient implements WAClient using the whatsmeow library.
type WhatsmeowClient struct {
	client    *whatsmeow.Client
	container *sqlstore.Container
	logger    *slog.Logger

	mu        sync.Mutex
	onMessage func(userID, text string)
}

// WhatsmeowConfig holds configuration for creating a WhatsmeowClient.
type WhatsmeowConfig struct {
	AuthDir string       // directory for the whatsapp.db session store
	Logger  *slog.Logger // structured logger
}

// NewWhatsmeowClient creates a WhatsmeowClient backed by whatsmeow.
// It initialises the SQL session store but does not connect.
func NewWhatsmeowClient(cfg WhatsmeowConfig) (*WhatsmeowClient, error) {
	if err := os.MkdirAll(cfg.AuthDir, 0o755); err != nil {
		return nil, fmt.Errorf("create auth dir: %w", err)
	}

	dbPath := filepath.Join(cfg.AuthDir, "whatsapp.db")
	dsn := fmt.Sprintf("file:%s?_foreign_keys=on", dbPath)

	container, err := sqlstore.New(context.Background(), "sqlite3", dsn, nil)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: %w", err)
	}

	device, err := container.GetFirstDevice(context.Background())
	if err != nil {
		_ = container.Close()
		return nil, fmt.Errorf("get device: %w", err)
	}

	client := whatsmeow.NewClient(device, nil)

	w := &WhatsmeowClient{
		client:    client,
		container: container,
		logger:    cfg.Logger,
	}

	client.AddEventHandler(w.handleEvent)

	return w, nil
}

// SetOnMessage sets the callback invoked when a WhatsApp text message arrives.
func (w *WhatsmeowClient) SetOnMessage(fn func(userID, text string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onMessage = fn
}

// Connect connects to WhatsApp. For unpaired devices it starts the QR code
// flow asynchronously, logging each code to slog.
func (w *WhatsmeowClient) Connect() error {
	if w.client.Store.ID == nil {
		return w.connectWithQR()
	}
	return w.client.Connect()
}

// connectWithQR handles the QR code pairing flow for new devices.
func (w *WhatsmeowClient) connectWithQR() error {
	qrChan, err := w.client.GetQRChannel(context.Background())
	if err != nil {
		return fmt.Errorf("get QR channel: %w", err)
	}

	if err := w.client.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	go func() {
		for evt := range qrChan {
			switch evt.Event {
			case "code":
				w.logger.Info("whatsapp QR code ready - scan with your phone",
					"qr", evt.Code,
				)
			case "success":
				w.logger.Info("whatsapp pairing successful")
			case "timeout":
				w.logger.Warn("whatsapp QR code timed out")
			default:
				if evt.Error != nil {
					w.logger.Error("whatsapp pairing error",
						"event", evt.Event,
						"error", evt.Error,
					)
				}
			}
		}
	}()

	return nil
}

// Disconnect disconnects from WhatsApp.
func (w *WhatsmeowClient) Disconnect() {
	w.client.Disconnect()
}

// SendText sends a plain-text message to the given JID string.
func (w *WhatsmeowClient) SendText(jid, text string) error {
	target, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("parse JID %q: %w", jid, err)
	}

	_, err = w.client.SendMessage(context.Background(), target, &waE2E.Message{
		Conversation: proto.String(text),
	})
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}

// IsConnected reports whether the whatsmeow client is connected.
func (w *WhatsmeowClient) IsConnected() bool {
	return w.client.IsConnected()
}

// Close cleans up the sqlstore container.
func (w *WhatsmeowClient) Close() error {
	return w.container.Close()
}

// handleEvent dispatches whatsmeow events.
func (w *WhatsmeowClient) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		w.handleMessage(v)
	}
}

// handleMessage extracts sender and text from an incoming message.
func (w *WhatsmeowClient) handleMessage(evt *events.Message) {
	if evt.Info.IsFromMe {
		return
	}

	text := extractText(evt.Message)
	if text == "" {
		return
	}

	sender := evt.Info.Sender.String()

	w.mu.Lock()
	fn := w.onMessage
	w.mu.Unlock()

	if fn != nil {
		fn(sender, text)
	}
}

// extractText pulls plain-text content from a WhatsApp message proto.
func extractText(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	if t := msg.GetConversation(); t != "" {
		return t
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetText()
	}
	return ""
}
