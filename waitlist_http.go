package lib

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/taubyte/go-sdk/database"
	"github.com/taubyte/go-sdk/event"
	httpevent "github.com/taubyte/go-sdk/http/event"
)

const waitlistStorageKey = "waitlist/entries"
const fikriaDataMatch = "/fikria/data"
const waitlistAdminKeyFallback = "J58WY5QkT1Wy5CN26v6TtwiB9lbBe9S7"

type waitlistEntry struct {
	ID                 string `json:"id"`
	FullName           string `json:"fullName"`
	Email              string `json:"email"`
	Phone              string `json:"phone,omitempty"`
	Role               string `json:"role,omitempty"`
	RoleOther          string `json:"roleOther,omitempty"`
	Organization       string `json:"organization,omitempty"`
	Message            string `json:"message,omitempty"`
	SubmittedAt        string `json:"submittedAt"`
	AccessEnabled      bool   `json:"accessEnabled"`
	AccessDisabled     bool   `json:"accessDisabled"`
	AiGenerationsGrant int    `json:"aiGenerationsGrant"`
	AiGenerationsUsed  int    `json:"aiGenerationsUsed"`
}

type waitlistPostBody struct {
	FullName     string `json:"fullName"`
	Email        string `json:"email"`
	Phone        string `json:"phone,omitempty"`
	Role         string `json:"role,omitempty"`
	RoleOther    string `json:"roleOther,omitempty"`
	Organization string `json:"organization,omitempty"`
	Message      string `json:"message,omitempty"`
	Website      string `json:"website,omitempty"`
}

type waitlistPatchBody struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	AccessEnabled      *bool  `json:"accessEnabled"`
	AiGenerationsGrant *int   `json:"aiGenerationsGrant"`
	ResetUsage         bool   `json:"resetUsage"`
	Delete             bool   `json:"delete"`
}

func adminKeyWant() string {
	if k := strings.TrimSpace(os.Getenv("WAITLIST_ADMIN_KEY")); k != "" {
		return k
	}
	return waitlistAdminKeyFallback
}

func adminKeyOK(h httpevent.Event) bool {
	want := adminKeyWant()
	wantB := []byte(want)
	if got, _ := h.Headers().Get("X-Waitlist-Admin-Key"); subtle.ConstantTimeCompare([]byte(strings.TrimSpace(got)), wantB) == 1 {
		return true
	}
	if q, err := h.Query().Get("admin_key"); err == nil {
		return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(q)), wantB) == 1
	}
	return false
}

func setWaitlistCors(h httpevent.Event) {
	_ = h.Headers().Set("Access-Control-Allow-Origin", "*")
	_ = h.Headers().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
	_ = h.Headers().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Waitlist-Admin-Key")
}

func wlJSONError(h httpevent.Event, status int, msg string) uint32 {
	setWaitlistCors(h)
	b, _ := json.Marshal(map[string]string{"error": msg})
	h.Headers().Set("Content-Type", "application/json")
	h.Write(b)
	h.Return(status)
	return 0
}

func wlJSONOK(h httpevent.Event, status int, payload interface{}) uint32 {
	setWaitlistCors(h)
	raw, _ := json.Marshal(payload)
	h.Headers().Set("Content-Type", "application/json")
	h.Write(raw)
	h.Return(status)
	return 0
}

func readWaitlist(db database.Database) ([]waitlistEntry, error) {
	val, err := db.Get(waitlistStorageKey)
	if err != nil || len(val) == 0 {
		return []waitlistEntry{}, nil
	}
	var list []waitlistEntry
	if err := json.Unmarshal(val, &list); err != nil {
		return nil, err
	}
	if list == nil {
		return []waitlistEntry{}, nil
	}
	return list, nil
}

func writeWaitlist(db database.Database, list []waitlistEntry) error {
	raw, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return db.Put(waitlistStorageKey, raw)
}

func findIdx(list []waitlistEntry, id, email string) int {
	id = strings.TrimSpace(id)
	email = strings.TrimSpace(strings.ToLower(email))
	for i, e := range list {
		if id != "" && e.ID == id {
			return i
		}
	}
	for i, e := range list {
		if email != "" && strings.EqualFold(strings.TrimSpace(e.Email), email) {
			return i
		}
	}
	return -1
}

//export apiWaitlist
func apiWaitlist(e event.Event) uint32 {
	h, err := e.HTTP()
	if err != nil {
		return 1
	}
	method, _ := h.Method()
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "OPTIONS":
		setWaitlistCors(h)
		h.Return(204)
		return 0
	case "POST":
		return wlPost(h)
	case "GET":
		return wlGet(h)
	case "PATCH":
		return wlPatch(h)
	default:
		return wlJSONError(h, 405, "méthode non autorisée")
	}
}

func wlPost(h httpevent.Event) uint32 {
	body, err := io.ReadAll(h.Body())
	if err != nil {
		return wlJSONError(h, 400, "corps invalide")
	}
	defer h.Body().Close()
	var req waitlistPostBody
	if err := json.Unmarshal(body, &req); err != nil {
		return wlJSONError(h, 400, "JSON invalide")
	}
	if strings.TrimSpace(req.Website) != "" {
		return wlJSONOK(h, 200, map[string]bool{"ok": true})
	}
	fullName := strings.TrimSpace(req.FullName)
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if fullName == "" || email == "" || !strings.Contains(email, "@") {
		return wlJSONError(h, 400, "nom et e-mail valides requis")
	}
	role := strings.TrimSpace(req.Role)
	if role == "Autre" && strings.TrimSpace(req.RoleOther) != "" {
		role = "Autre — " + strings.TrimSpace(req.RoleOther)
	}
	entry := waitlistEntry{
		ID: fmt.Sprintf("%d", time.Now().UnixNano()), FullName: fullName, Email: email,
		Phone: strings.TrimSpace(req.Phone), Role: role,
		Organization: strings.TrimSpace(req.Organization), Message: strings.TrimSpace(req.Message),
		SubmittedAt: time.Now().UTC().Format(time.RFC3339),
	}
	db, err := database.New(fikriaDataMatch)
	if err != nil {
		return wlJSONError(h, 500, "base indisponible")
	}
	defer db.Close()
	list, err := readWaitlist(db)
	if err != nil {
		return wlJSONError(h, 500, "lecture waitlist")
	}
	list = append(list, entry)
	if err := writeWaitlist(db, list); err != nil {
		return wlJSONError(h, 500, "enregistrement waitlist")
	}
	return wlJSONOK(h, 200, map[string]interface{}{"ok": true, "id": entry.ID})
}

func wlGet(h httpevent.Event) uint32 {
	if !adminKeyOK(h) {
		return wlJSONError(h, 401, "clé admin waitlist requise")
	}
	db, err := database.New(fikriaDataMatch)
	if err != nil {
		return wlJSONError(h, 500, "base indisponible")
	}
	defer db.Close()
	list, err := readWaitlist(db)
	if err != nil {
		return wlJSONError(h, 500, "lecture waitlist")
	}
	dirty := false
	for i := range list {
		if list[i].AiGenerationsGrant > 0 && !list[i].AccessDisabled && !list[i].AccessEnabled {
			list[i].AccessEnabled = true
			dirty = true
		}
	}
	if dirty {
		_ = writeWaitlist(db, list)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].SubmittedAt > list[j].SubmittedAt })
	return wlJSONOK(h, 200, map[string]interface{}{"entries": list})
}

func wlPatch(h httpevent.Event) uint32 {
	if !adminKeyOK(h) {
		return wlJSONError(h, 401, "clé admin waitlist requise")
	}
	body, err := io.ReadAll(h.Body())
	if err != nil {
		return wlJSONError(h, 400, "corps invalide")
	}
	defer h.Body().Close()
	var req waitlistPatchBody
	if err := json.Unmarshal(body, &req); err != nil {
		return wlJSONError(h, 400, "JSON invalide")
	}
	db, err := database.New(fikriaDataMatch)
	if err != nil {
		return wlJSONError(h, 500, "base indisponible")
	}
	defer db.Close()
	list, err := readWaitlist(db)
	if err != nil {
		return wlJSONError(h, 500, "lecture waitlist")
	}
	idx := findIdx(list, req.ID, req.Email)
	if idx < 0 {
		return wlJSONError(h, 404, "inscription introuvable")
	}
	if req.Delete {
		removed := list[idx]
		list = append(list[:idx], list[idx+1:]...)
		_ = writeWaitlist(db, list)
		return wlJSONOK(h, 200, map[string]interface{}{"ok": true, "deleted": true, "email": removed.Email})
	}
	if req.AccessEnabled != nil {
		list[idx].AccessEnabled = *req.AccessEnabled
	}
	if req.AiGenerationsGrant != nil {
		list[idx].AiGenerationsGrant = *req.AiGenerationsGrant
		if *req.AiGenerationsGrant > 0 {
			list[idx].AccessEnabled = true
			list[idx].AccessDisabled = false
		}
	}
	if req.ResetUsage {
		list[idx].AiGenerationsUsed = 0
	}
	if err := writeWaitlist(db, list); err != nil {
		return wlJSONError(h, 500, "mise à jour")
	}
	return wlJSONOK(h, 200, map[string]interface{}{"ok": true, "entry": list[idx]})
}
