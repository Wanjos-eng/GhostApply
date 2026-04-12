package main

import (
	"log"
	"os"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

// IMAPListener handles connections and message extractions
type IMAPListener struct {
	client *client.Client
}

// NewIMAPListener connects to IMAP via TLS using .env credentials
// Default config focuses on robust TLS support.
func NewIMAPListener() (*IMAPListener, error) {
	addr := os.Getenv("IMAP_SERVER") + ":" + os.Getenv("IMAP_PORT")
	
	log.Println("IMAP: Connecting to server...")
	// Task 51: Secure TLS connection
	c, err := client.DialTLS(addr, nil)
	if err != nil {
		return nil, err
	}

	log.Println("IMAP: Logging in...")
	if err := c.Login(os.Getenv("IMAP_USER"), os.Getenv("IMAP_PASS")); err != nil {
		return nil, err
	}
	
	log.Println("IMAP: Logged in successfully.")

	return &IMAPListener{client: c}, nil
}

// FetchUnseenEmailBodies retrieves the textual bodies of all UNSEEN emails
// Task 52: Extract unseen emails
func (l *IMAPListener) FetchUnseenEmailBodies() (map[uint32]string, error) {
	mbox, err := l.client.Select("INBOX", false)
	if err != nil {
		return nil, err
	}

	if mbox.Messages == 0 {
		return nil, nil
	}

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	
	seqNums, err := l.client.Search(criteria)
	if err != nil {
		return nil, err
	}

	if len(seqNums) == 0 {
		return nil, nil // None unseen
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(seqNums...)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- l.client.Fetch(seqset, items, messages)
	}()

	results := make(map[uint32]string)

	for msg := range messages {
		r := msg.GetBody(section)
		if r == nil {
			log.Println("IMAP: Server didn't return message body")
			continue
		}

		// Create a new mail reader
		mr, err := mail.CreateReader(r)
		if err != nil {
			log.Printf("IMAP: Failed creating mail reader: %v", err)
			continue
		}
		
		var bodyBuilder strings.Builder

		// Process each text part of the email
		for {
			p, err := mr.NextPart()
			if err != nil {
				break // EOF
			}
			switch h := p.Header.(type) {
			case *mail.InlineHeader:
				// Only extract text/plain or text/html
				contentType, _, _ := h.ContentType()
				if strings.HasPrefix(contentType, "text/plain") {
					buf := make([]byte, 1024)
					for {
						n, err := p.Body.Read(buf)
						bodyBuilder.Write(buf[:n])
						if err != nil {
							break
						}
					}
				}
			}
		}

		text := strings.TrimSpace(bodyBuilder.String())
		if text != "" {
			results[msg.SeqNum] = text
		}
	}

	if err := <-done; err != nil {
		return nil, err
	}

	return results, nil
}

// MarkAsSeen tags explicit emails so they don't get processed twice
func (l *IMAPListener) MarkAsSeen(seqNum uint32) error {
	seqset := new(imap.SeqSet)
	seqset.AddNum(seqNum)
	
	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.SeenFlag}
	
	if err := l.client.Store(seqset, item, flags, nil); err != nil {
		return err
	}
	return nil
}
