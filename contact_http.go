package lib

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/taubyte/go-sdk/database"
	"github.com/taubyte/go-sdk/event"
	httpevent "github.com/taubyte/go-sdk/http/event"
)

const contactStorageKey = "contact/messages"

type contactMessage struct {
	ID          string `json:"id"`
	FullName    string `json:"fullName"`
	Email       string `json:"email"`
	Subject     string `json:"subject"`
	Message     string `json:"message"`
	SubmittedAt string `json:"submittedAt"`
}

type contactPostBody struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Subject  string `json:"subject"`
	Message  string `json:"message"`
	Website  string `json:"website,omitempty"`
}

func setContactCors(h httpevent.Event) {
	_ = h.Headers().Set("Access-Control-Allow-Origin", "*")
	_ = h.Headers().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	_ = h.Headers().Set("Access-Control-Allow-Headers", "Content-Type, X-Waitlist-Admin-Key")
}

//export apiContact
func apiContact(e event.Event) uint32 {
	h, err := e.HTTP()
	if err != nil {
		return 1
	}
	method, _ := h.Method()
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "OPTIONS":
		setContactCors(h)
		h.Return(204)
		return 0
	case "POST":
		return contactPost(h)
	case "GET":
		return contactGet(h)
	default:
		setContactCors(h)
		b, _ := json.Marshal(map[string]string{"error": "méthode non autorisée"})
		h.Headers().Set("Content-Type", "application/json")
		h.Write(b)
		h.Return(405)
		return 0
	}
}

func contactPost(h httpevent.Event) uint32 {
	body, err := io.ReadAll(h.Body())
	if err != nil {
		return contactErr(h, 400, "corps invalide")
	}
	defer h.Body().Close()
	var req contactPostBody
	if err := json.Unmarshal(body, &req); err != nil {
		return contactErr(h, 400, "JSON invalide")
	}
	if strings.TrimSpace(req.Website) != "" {
		return contactOK(h, map[string]bool{"ok": true})
	}
	fullName := strings.TrimSpace(req.FullName)
	email := strings.TrimSpace(strings.ToLower(req.Email))
	subject := strings.TrimSpace(req.Subject)
	message := strings.TrimSpace(req.Message)
	if fullName == "" || email == "" || !strings.Contains(email, "@") {
		return contactErr(h, 400, "nom et e-mail valides requis")
	}
	if subject == "" {
		return contactErr(h, 400, "sujet requis")
	}
	if len(message) < 10 {
		return contactErr(h, 400, "message trop court")
	}
	entry := contactMessage{
		ID: fmt.Sprintf("%d", time.Now().UnixNano()), FullName: fullName, Email: email,
		Subject: subject, Message: message, SubmittedAt: time.Now().UTC().Format(time.RFC3339),
	}
	db, err := database.New(fikriaDataMatch)
	if err != nil {
		return contactErr(h, 500, "base indisponible")
	}
	defer db.Close()
	val, _ := db.Get(contactStorageKey)
	var list []contactMessage
	if len(val) > 0 {
		_ = json.Unmarshal(val, &list)
	}
	list = append(list, entry)
	raw, _ := json.Marshal(list)
	if err := db.Put(contactStorageKey, raw); err != nil {
		return contactErr(h, 500, "enregistrement message")
	}
	return contactOK(h, map[string]interface{}{"ok": true, "id": entry.ID, "mailConfigured": false})
}

func contactGet(h httpevent.Event) uint32 {
	if !adminKeyOK(h) {
		return contactErr(h, 401, "clé admin requise")
	}
	db, err := database.New(fikriaDataMatch)
	if err != nil {
		return contactErr(h, 500, "base indisponible")
	}
	defer db.Close()
	val, err := db.Get(contactStorageKey)
	if err != nil || len(val) == 0 {
		return contactOK(h, map[string]interface{}{"messages": []contactMessage{}})
	}
	var list []contactMessage
	if err := json.Unmarshal(val, &list); err != nil {
		return contactErr(h, 500, "lecture messages")
	}
	return contactOK(h, map[string]interface{}{"messages": list})
}

func contactErr(h httpevent.Event, status int, msg string) uint32 {
	setContactCors(h)
	b, _ := json.Marshal(map[string]string{"error": msg})
	h.Headers().Set("Content-Type", "application/json")
	h.Write(b)
	h.Return(status)
	return 0
}

func contactOK(h httpevent.Event, payload interface{}) uint32 {
	setContactCors(h)
	raw, _ := json.Marshal(payload)
	h.Headers().Set("Content-Type", "application/json")
	h.Write(raw)
	h.Return(200)
	return 0
}
