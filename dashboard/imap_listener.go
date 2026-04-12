package main

import (
	"log"
	"os"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

func composeIMAPAddr(server, port string) string {
	srv := strings.TrimSpace(server)
	if srv == "" {
		srv = "imap.gmail.com"
	}

	if strings.Contains(srv, ":") {
		return srv
	}

	p := strings.TrimSpace(port)
	if p == "" {
		p = "993"
	}

	return srv + ":" + p
}

// IMAPListener concentra a conexão IMAP e a extração das mensagens.
type IMAPListener struct {
	client *client.Client
}

// NewIMAPListener conecta ao IMAP via TLS usando as credenciais do .env.
// A configuração padrão prioriza estabilidade de TLS.
func NewIMAPListener() (*IMAPListener, error) {
	addr := composeIMAPAddr(os.Getenv("IMAP_SERVER"), os.Getenv("IMAP_PORT"))
	
	log.Println("IMAP: Connecting to server...")
	// Conexão IMAP com TLS seguro.
	c, err := client.DialTLS(addr, nil)
	if err != nil {
		return nil, err
	}

	log.Println("IMAP: Logging in...")
	if err := c.Login(os.Getenv("IMAP_USER"), os.Getenv("IMAP_PASS")); err != nil {
		_ = c.Logout()
		return nil, err
	}
	
	log.Println("IMAP: Logged in successfully.")

	return &IMAPListener{client: c}, nil
}

// FetchUnseenEmailBodies lê o corpo textual de todos os emails não vistos.
// Extrai emails não lidos.
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
		return nil, nil // Nenhuma mensagem não lida.
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

		// Cria um leitor de e-mail para o corpo retornado.
		mr, err := mail.CreateReader(r)
		if err != nil {
			log.Printf("IMAP: Failed creating mail reader: %v", err)
			continue
		}
		
		var bodyBuilder strings.Builder

		// Processa cada parte textual do email.
		for {
			p, err := mr.NextPart()
			if err != nil {
				break // Fim do conteúdo.
			}
			switch h := p.Header.(type) {
			case *mail.InlineHeader:
				// Extrai apenas text/plain ou text/html.
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

// MarkAsSeen marca os emails já processados para evitar duplicidade.
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
