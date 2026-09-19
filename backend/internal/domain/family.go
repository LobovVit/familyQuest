package domain

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"
	"time"
)

// FamilyEntry is one family activity aggregate. Definitions and progress are saved
// together with optimistic locking, so concurrent devices cannot overwrite work.
type FamilyEntry struct {
	ID        int64     `json:"id"`
	Version   int       `json:"version"`
	Kind      string    `json:"kind"`
	AuthorID  int64     `json:"authorId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Archived  bool      `json:"archived"`
	Approved  bool      `json:"approved"`
	FamilyDraft
	Events []FamilyEvent `json:"events"`
}
type FamilyDraft struct {
	Title          string       `json:"title"`
	Description    string       `json:"description"`
	Date           string       `json:"date"`
	ParticipantIDs []int64      `json:"participantIds"`
	Steps          []FamilyStep `json:"steps"`
	Weekdays       []int        `json:"weekdays"`
	Reminder       string       `json:"reminder"`
	EasyVersion    string       `json:"easyVersion"`
	Value          string       `json:"value"`
	Agreement      string       `json:"agreement"`
	NextActivity   string       `json:"nextActivity"`
	Photo          string       `json:"photo"`
	Audio          string       `json:"audio"`
}
type FamilyStep struct {
	Title         string `json:"title"`
	ParticipantID int64  `json:"participantId"`
}
type FamilyEvent struct {
	ActorID   int64     `json:"actorId"`
	Kind      string    `json:"kind"`
	Date      string    `json:"date"`
	Step      int       `json:"step"`
	Note      string    `json:"note"`
	Easy      bool      `json:"easy"`
	CreatedAt time.Time `json:"createdAt"`
}
type FamilyCommand struct {
	Version int    `json:"version"`
	Action  string `json:"action"`
	Date    string `json:"date"`
	Step    int    `json:"step"`
	Note    string `json:"note"`
	Easy    bool   `json:"easy"`
}

func IsFamilyMember(p Principal) bool {
	return p.ParticipantID > 0 && (p.Role == RoleParent || p.Role == RoleChild)
}
func (e FamilyEntry) CanEdit(p Principal) bool {
	return IsFamilyMember(p) && (p.IsParent() || e.AuthorID == p.ParticipantID)
}
func familyInvalid(message string) error { return fmt.Errorf("%w: %s", ErrInvalidInput, message) }
func ValidFamilyDate(value string) bool {
	d, err := time.Parse("2006-01-02", value)
	return err == nil && d.Year() >= 2000 && d.Year() <= 2100
}
func (e FamilyEntry) Validate() error {
	if !slices.Contains([]string{"adventure", "habit", "value", "skill", "proposal", "council", "memory"}, e.Kind) {
		return familyInvalid("неизвестный раздел")
	}
	if strings.TrimSpace(e.Title) == "" || len([]rune(e.Title)) > 160 {
		return familyInvalid("название: от 1 до 160 символов")
	}
	for _, text := range []string{e.Description, e.Agreement, e.NextActivity, e.EasyVersion} {
		if len([]rune(text)) > 4000 {
			return familyInvalid("текст длиннее 4000 символов")
		}
	}
	if e.Kind == "value" && (strings.TrimSpace(e.Value) == "" || strings.TrimSpace(e.Description) == "") {
		return familyInvalid("укажите ценность и конкретный поступок")
	}
	if len([]rune(e.Value)) > 100 {
		return familyInvalid("слишком длинное название ценности")
	}
	if e.Date != "" && !ValidFamilyDate(e.Date) {
		return familyInvalid("некорректная дата")
	}
	if (e.Kind == "memory" || e.Kind == "council" || e.Kind == "adventure" || e.Kind == "proposal") && e.Date == "" {
		return familyInvalid("укажите дату")
	}
	if len(e.ParticipantIDs) > 50 || len(e.Steps) > 30 || len(e.Events) > 10000 {
		return familyInvalid("слишком много участников, шагов или отметок")
	}
	seen := map[int64]bool{}
	for _, id := range e.ParticipantIDs {
		if id <= 0 || seen[id] {
			return familyInvalid("некорректные участники")
		}
		seen[id] = true
	}
	for _, step := range e.Steps {
		if strings.TrimSpace(step.Title) == "" || len([]rune(step.Title)) > 500 || step.ParticipantID < 0 {
			return familyInvalid("некорректный шаг")
		}
		if step.ParticipantID > 0 && len(e.ParticipantIDs) > 0 && !seen[step.ParticipantID] {
			return familyInvalid("ответственный должен участвовать в занятии")
		}
	}
	if (e.Kind == "adventure" || e.Kind == "proposal") && len(e.Steps) == 0 {
		return familyInvalid("добавьте хотя бы один шаг")
	}
	if e.Kind == "habit" && (len(e.Weekdays) == 0 || len(e.Weekdays) > 7) {
		return familyInvalid("выберите дни привычки")
	}
	days := map[int]bool{}
	for _, day := range e.Weekdays {
		if day < 0 || day > 6 || days[day] {
			return familyInvalid("некорректные дни недели")
		}
		days[day] = true
	}
	if e.Reminder != "" {
		if _, err := time.Parse("15:04", e.Reminder); err != nil {
			return familyInvalid("некорректное время напоминания")
		}
	}
	if e.Kind != "memory" && (e.Photo != "" || e.Audio != "") {
		return familyInvalid("медиа доступны в воспоминаниях")
	}
	if err := validateFamilyMedia(e.Photo, false); err != nil {
		return err
	}
	if err := validateFamilyMedia(e.Audio, true); err != nil {
		return err
	}
	for _, event := range e.Events {
		if event.ActorID <= 0 || !ValidFamilyDate(event.Date) || len([]rune(event.Note)) > 2000 || event.CreatedAt.IsZero() {
			return familyInvalid("некорректная история")
		}
		if !slices.Contains([]string{"checkin", "step", "stage", "reflection", "repeat"}, event.Kind) {
			return familyInvalid("некорректный вид отметки")
		}
		if event.Kind == "step" && (event.Step < 0 || event.Step >= len(e.Steps)) {
			return familyInvalid("шаг истории не существует")
		}
		if event.Kind == "stage" && (event.Step < 1 || event.Step > 4) {
			return familyInvalid("некорректный этап навыка")
		}
	}
	return nil
}
func validateFamilyMedia(value string, audio bool) error {
	if value == "" {
		return nil
	}
	header, data, ok := strings.Cut(value, ",")
	if !ok || len(data) > 700000 {
		return familyInvalid("медиа должно быть меньше 500 КБ")
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(raw) == 0 || len(raw) > 500000 {
		return familyInvalid("некорректное медиа")
	}
	matches := func(signature []byte) bool { return bytes.HasPrefix(raw, signature) }
	if !audio && ((header == "data:image/jpeg;base64" && matches([]byte{0xff, 0xd8, 0xff})) || (header == "data:image/png;base64" && matches([]byte{137, 80, 78, 71, 13, 10, 26, 10}))) {
		return nil
	}
	if audio {
		valid := (header == "data:audio/mpeg;base64" && (matches([]byte("ID3")) || (len(raw) > 1 && raw[0] == 0xff && raw[1]&0xe0 == 0xe0))) ||
			(header == "data:audio/wav;base64" && len(raw) > 12 && matches([]byte("RIFF")) && string(raw[8:12]) == "WAVE") ||
			(header == "data:audio/ogg;base64" && matches([]byte("OggS"))) ||
			(header == "data:audio/webm;base64" && matches([]byte{0x1a, 0x45, 0xdf, 0xa3})) ||
			(header == "data:audio/mp4;base64" && len(raw) > 12 && string(raw[4:8]) == "ftyp")
		if valid {
			return nil
		}
	}
	return familyInvalid("поддерживаются JPEG/PNG и MP3/WAV/OGG/WebM/M4A")
}

// ApplyFamilyCommand never trusts an actor, approval or progress supplied by a client.
func (e *FamilyEntry) ApplyFamilyCommand(actor Principal, cmd FamilyCommand, now time.Time) error {
	if !IsFamilyMember(actor) {
		return ErrForbidden
	}
	if cmd.Version != e.Version {
		return ErrConflict
	}
	switch cmd.Action {
	case "archive", "restore":
		if !e.CanEdit(actor) {
			return ErrForbidden
		}
		e.Archived = cmd.Action == "archive"
		return nil
	case "approve":
		if !actor.IsParent() {
			return ErrForbidden
		}
		if e.Kind != "proposal" || e.Archived {
			return ErrConflict
		}
		e.Approved = true
		return nil
	}
	if e.Archived || (e.Kind == "proposal" && !e.Approved) {
		return ErrConflict
	}
	if !ValidFamilyDate(cmd.Date) || cmd.Date > now.UTC().Add(24*time.Hour).Format("2006-01-02") {
		return familyInvalid("укажите дату не позднее сегодняшней")
	}
	if len([]rune(cmd.Note)) > 2000 {
		return familyInvalid("заметка длиннее 2000 символов")
	}
	if len(e.Events) >= 10000 {
		return familyInvalid("архивируйте карточку и создайте новую")
	}
	event := FamilyEvent{ActorID: actor.ParticipantID, Kind: cmd.Action, Date: cmd.Date, Step: cmd.Step, Note: strings.TrimSpace(cmd.Note), Easy: cmd.Easy, CreatedAt: now.UTC()}
	if cmd.Action != "reflection" && cmd.Action != "repeat" && len(e.ParticipantIDs) > 0 && !slices.Contains(e.ParticipantIDs, actor.ParticipantID) {
		return ErrForbidden
	}
	switch cmd.Action {
	case "checkin":
		if e.Kind != "habit" && e.Kind != "value" {
			return ErrInvalidInput
		}
		for _, v := range e.Events {
			if v.Kind == cmd.Action && v.Date == cmd.Date && v.ActorID == actor.ParticipantID {
				return nil
			}
		}
	case "step":
		if (e.Kind != "adventure" && e.Kind != "proposal") || cmd.Step < 0 || cmd.Step >= len(e.Steps) {
			return ErrInvalidInput
		}
		step := e.Steps[cmd.Step]
		if step.ParticipantID > 0 && step.ParticipantID != actor.ParticipantID {
			return ErrForbidden
		}
		for _, v := range e.Events {
			if v.Kind == "step" && v.Step == cmd.Step {
				return nil
			}
		}
	case "stage":
		if e.Kind != "skill" {
			return ErrInvalidInput
		}
		stage := 0
		for _, v := range e.Events {
			if v.Kind == "stage" && v.ActorID == actor.ParticipantID && v.Step > stage {
				stage = v.Step
			}
		}
		if cmd.Step != stage+1 || cmd.Step > 4 {
			return ErrConflict
		}
	case "reflection":
		if event.Note == "" {
			return familyInvalid("добавьте текст заметки")
		}
	case "repeat":
		for _, v := range e.Events {
			if v.Kind == "repeat" && v.ActorID == actor.ParticipantID {
				return nil
			}
		}
	default:
		return ErrInvalidInput
	}
	e.Events = append(e.Events, event)
	return nil
}
