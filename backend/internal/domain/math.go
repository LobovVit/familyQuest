package domain

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"time"
)

type MathSettings struct {
	Operation     string `json:"operation"`
	Level         string `json:"level"`
	AnswerMode    string `json:"answerMode"`
	DivisionMode  string `json:"divisionMode"`
	AllowNegative bool   `json:"allowNegative"`
}
type MathQuestion struct {
	Left    int   `json:"left"`
	Right   int   `json:"right"`
	Options []int `json:"options"`
}
type MathAnswer struct {
	Index   int            `json:"index"`
	Values  map[string]int `json:"values"`
	Correct bool           `json:"correct"`
	Stars   int            `json:"stars"`
	Date    string         `json:"date"`
}

const MathDailyStarLimit = 40
const PreviousMathDailyStarLimit = 30

type MathSession struct {
	PolicyVersion int            `json:"policyVersion,omitempty"`
	RewardVersion int            `json:"rewardVersion,omitempty"`
	Closed        bool           `json:"closed"`
	ID            string         `json:"id"`
	ParticipantID int64          `json:"participantId"`
	Settings      MathSettings   `json:"settings"`
	Questions     []MathQuestion `json:"questions"`
	Answers       []MathAnswer   `json:"answers"`
	CreatedAt     time.Time      `json:"createdAt"`
}
type MathField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Width int    `json:"width"`
	Shift int    `json:"shift"`
}
type MathFeedback struct {
	Index    int            `json:"index"`
	Correct  bool           `json:"correct"`
	Stars    int            `json:"stars"`
	Expected map[string]int `json:"expected"`
	Fields   []MathField    `json:"fields"`
	Question MathQuestion   `json:"question"`
}
type MathView struct {
	Finished   bool          `json:"finished"`
	ID         string        `json:"id"`
	Settings   MathSettings  `json:"settings"`
	Index      int           `json:"index"`
	Total      int           `json:"total"`
	Correct    int           `json:"correct"`
	Stars      int           `json:"stars"`
	BestStreak int           `json:"bestStreak"`
	CreatedAt  time.Time     `json:"createdAt"`
	Question   *MathQuestion `json:"question,omitempty"`
	Fields     []MathField   `json:"fields"`
	Feedback   *MathFeedback `json:"feedback,omitempty"`
}

func (s MathSettings) Validate() error {
	if !slices.Contains([]string{"+", "-", "*", ":"}, s.Operation) || !slices.Contains([]string{"easy", "medium", "hard", "columnar"}, s.Level) || !slices.Contains([]string{"input", "choice"}, s.AnswerMode) || !slices.Contains([]string{"result", "steps", "full"}, s.DivisionMode) || (s.Level == "columnar" && s.AnswerMode != "input") {
		return ErrInvalidInput
	}
	return nil
}
func (s MathSettings) Count() int {
	switch s.Level {
	case "medium":
		return 12
	case "hard":
		return 15
	default:
		return 8
	}
}
func (s MathSettings) legacyStars() int {
	switch s.Level {
	case "easy":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}

// Stars rewards the selected work, not speed or a streak. Choice has less weight.
func (s MathSettings) Stars() int { return s.previousStars() + 1 }

func (s MathSettings) previousStars() int {
	stars := s.legacyStars()
	if s.Level == "columnar" {
		if s.Operation == "*" {
			return 4
		}
		if s.Operation == ":" {
			switch s.DivisionMode {
			case "full":
				return 5
			case "steps":
				return 4
			default:
				return 3
			}
		}
		return 3
	}
	if s.Operation == "*" || s.Operation == ":" {
		stars++
	}
	if s.AnswerMode == "choice" {
		stars = maxInt(1, stars-1)
	}
	return stars
}

func NewMathSession(id string, p int64, settings MathSettings, now time.Time) MathSession {
	s := MathSession{RewardVersion: 3, ID: id, ParticipantID: p, Settings: settings, CreatedAt: now.UTC(), Answers: []MathAnswer{}}
	max := 10
	if settings.Level == "medium" {
		max = 25
	}
	if settings.Level == "hard" {
		max = 100
	}
	for range settings.Count() {
		a, b := rand.IntN(max+1), rand.IntN(max+1)
		if settings.Level == "columnar" {
			a = 12 + rand.IntN(988)
			b = 12 + rand.IntN(988)
		}
		switch settings.Operation {
		case "-":
			if (!settings.AllowNegative || settings.Level == "columnar") && a < b {
				a, b = b, a
			}
		case "*":
			a = rand.IntN(min(max, 12) + 1)
			b = rand.IntN(min(max, 12) + 1)
			if settings.Level == "columnar" {
				a = 12 + rand.IntN(88)
				b = 2 + rand.IntN(24)
			}
		case ":":
			b = 1 + rand.IntN(min(max, 12))
			result := rand.IntN(max + 1)
			if settings.Level == "columnar" {
				b = 2 + rand.IntN(8)
				result = 2 + rand.IntN(98)
			}
			a = b * result
		}
		q := MathQuestion{Left: a, Right: b, Options: []int{}}
		if settings.AnswerMode == "choice" {
			answer := q.Result(settings.Operation)
			q.Options = append(q.Options, answer)
			for len(q.Options) < 4 {
				v := answer + rand.IntN(17) - 8
				if !settings.AllowNegative {
					v = maxInt(0, v)
				}
				if !slices.Contains(q.Options, v) {
					q.Options = append(q.Options, v)
				}
			}
			rand.Shuffle(4, func(i, j int) { q.Options[i], q.Options[j] = q.Options[j], q.Options[i] })
		}
		s.Questions = append(s.Questions, q)
	}
	return s
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func (q MathQuestion) Result(op string) int {
	switch op {
	case "+":
		return q.Left + q.Right
	case "-":
		return q.Left - q.Right
	case "*":
		return q.Left * q.Right
	case ":":
		if q.Right != 0 {
			return q.Left / q.Right
		}
	}
	return 0
}
func digits(v int) int { return len(strconv.Itoa(v)) }
func (q MathQuestion) Work(s MathSettings) ([]MathField, map[string]int) {
	fields := []MathField{}
	expected := map[string]int{}
	add := func(key, label string, value, width, shift int) {
		fields = append(fields, MathField{key, label, width, shift})
		expected[key] = value
	}
	n := maxInt(digits(q.Left), digits(q.Right))
	if s.Level == "columnar" {
		switch s.Operation {
		case "+", "-":
			a, b, carry := q.Left, q.Right, 0
			for i := 0; i < n; i++ {
				next := 0
				if s.Operation == "+" {
					next = (a%10 + b%10 + carry) / 10
				} else if a%10-carry < b%10 {
					next = 1
				}
				label := "Перенос"
				if s.Operation == "-" {
					label = "Заём"
				}
				add(fmt.Sprintf("carry_%d", i), fmt.Sprintf("%s из разряда %d (0, если нет)", label, i+1), next, 1, 0)
				carry = next
				a /= 10
				b /= 10
			}
		case "*":
			b := q.Right
			for i := 0; b > 0; i++ {
				digit := b % 10
				a, carry := q.Left, 0
				for j := 0; j < digits(q.Left); j++ {
					carry = (a%10*digit + carry) / 10
					add(fmt.Sprintf("mulcarry_%d_%d", i, j), fmt.Sprintf("Строка %d: перенос из разряда %d", i+1, j+1), carry, 1, 0)
					a /= 10
				}
				add(fmt.Sprintf("partial_%d", i), fmt.Sprintf("Умножь %d на цифру %d", q.Left, digit), q.Left*digit, digits(q.Left)+1, i)
				b /= 10
			}
		case ":":
			if s.DivisionMode != "result" {
				partial, started, step := 0, false, 0
				for _, c := range strconv.Itoa(q.Left) {
					d := int(c - '0')
					partial = partial*10 + d
					if !started && partial < q.Right {
						continue
					}
					if started && s.DivisionMode == "full" {
						add(fmt.Sprintf("bring_%d", step), fmt.Sprintf("Шаг %d: снесённая цифра", step+1), d, 1, 0)
					}
					started = true
					product := partial / q.Right * q.Right
					remainder := partial - product
					add(fmt.Sprintf("product_%d", step), fmt.Sprintf("Шаг %d: произведение под делимым", step+1), product, n, 0)
					add(fmt.Sprintf("remainder_%d", step), fmt.Sprintf("Шаг %d: остаток после вычитания", step+1), remainder, n, 0)
					partial = remainder
					step++
				}
			}
		}
	}
	add("answer", "Ответ", q.Result(s.Operation), maxInt(n, digits(q.Result(s.Operation))), 0)
	return fields, expected
}
func (s *MathSession) Submit(index int, values map[string]int, now time.Time) error {
	if index < 0 || index >= len(s.Questions) {
		return ErrInvalidInput
	}
	if index < len(s.Answers) {
		return nil
	} // A network retry returns the saved result, never another reward.
	if s.Closed {
		return ErrConflict
	}
	if index != len(s.Answers) {
		return ErrConflict
	}
	_, expected := s.Questions[index].Work(s.Settings)
	if len(values) != len(expected) {
		return ErrInvalidInput
	}
	correct := true
	for k, want := range expected {
		v, ok := values[k]
		if !ok || v < -1000000 || v > 1000000 {
			return ErrInvalidInput
		}
		if v != want {
			correct = false
		}
	}
	stars := 0
	if correct {
		stars = s.Settings.Stars()
		if s.RewardVersion == 2 {
			stars = s.Settings.previousStars()
		}
		if s.RewardVersion == 0 {
			stars = s.Settings.legacyStars()
		}
	}
	s.Answers = append(s.Answers, MathAnswer{index, values, correct, stars, now.In(time.FixedZone("Europe/Minsk", 3*60*60)).Format("2006-01-02")})
	return nil
}
func (s MathSession) View() MathView {
	v := MathView{Finished: s.Closed || len(s.Answers) == len(s.Questions), ID: s.ID, Settings: s.Settings, Index: len(s.Answers), Total: len(s.Questions), CreatedAt: s.CreatedAt, Fields: []MathField{}}
	streak := 0
	for _, a := range s.Answers {
		v.Stars += a.Stars
		if a.Correct {
			v.Correct++
			streak++
		} else {
			streak = 0
		}
		v.BestStreak = maxInt(v.BestStreak, streak)
	}
	if !v.Finished && v.Index < v.Total {
		q := s.Questions[v.Index]
		v.Question = &q
		v.Fields, _ = q.Work(s.Settings)
	}
	if v.Index > 0 {
		a := s.Answers[v.Index-1]
		q := s.Questions[a.Index]
		fields, expected := q.Work(s.Settings)
		v.Feedback = &MathFeedback{a.Index, a.Correct, a.Stars, expected, fields, q}
	}
	return v
}

func (s MathSession) Validate() error {
	if (s.RewardVersion != 0 && s.RewardVersion != 2 && s.RewardVersion != 3) || len(s.ID) != 32 || s.ParticipantID <= 0 || s.CreatedAt.IsZero() || s.Settings.Validate() != nil || len(s.Questions) != s.Settings.Count() || len(s.Answers) > len(s.Questions) {
		return ErrInvalidInput
	}
	for _, c := range s.ID {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return ErrInvalidInput
		}
	}
	for _, q := range s.Questions {
		if q.Left < 0 || q.Left > 2000 || q.Right < 0 || q.Right > 1000 || (s.Settings.Operation == ":" && (q.Right == 0 || q.Left%q.Right != 0)) || (s.Settings.Operation == "-" && (!s.Settings.AllowNegative || s.Settings.Level == "columnar") && q.Left < q.Right) {
			return ErrInvalidInput
		}
		if s.Settings.AnswerMode == "choice" {
			if len(q.Options) != 4 || !slices.Contains(q.Options, q.Result(s.Settings.Operation)) {
				return ErrInvalidInput
			}
			for i, v := range q.Options {
				if slices.Contains(q.Options[:i], v) {
					return ErrInvalidInput
				}
			}
		} else if len(q.Options) != 0 {
			return ErrInvalidInput
		}
	}
	check := s
	check.Closed = false
	check.Answers = []MathAnswer{}
	for i, a := range s.Answers {
		d, err := time.Parse("2006-01-02", a.Date)
		if err != nil || !ValidFamilyDate(a.Date) || a.Index != i {
			return ErrInvalidInput
		}
		if err = check.Submit(i, a.Values, d); err != nil {
			return err
		}
		got := check.Answers[i]
		if got.Correct != a.Correct || a.Stars < 0 || a.Stars > got.Stars || (s.RewardVersion == 0 && got.Stars != a.Stars) {
			return ErrInvalidInput
		}
	}
	return nil
}

// AwardAnswer applies policy to a newly recorded answer. The repository supplies
// the daily total while holding the participant lock, then persists both changes.
func (s *MathSession) AwardAnswer(index, earned int) *ActivityReward {
	a := s.Answers[index]
	if s.RewardVersion >= 2 {
		limit := MathDailyStarLimit
		if s.RewardVersion == 2 {
			limit = PreviousMathDailyStarLimit
		}
		a.Stars = min(a.Stars, max(0, limit-earned))
		s.Answers[index] = a
	}
	if a.Stars == 0 {
		return nil
	}
	return &ActivityReward{Source: "math", SourceKey: fmt.Sprintf("%s:%d", s.ID, index), ParticipantID: s.ParticipantID, Date: a.Date, Stars: a.Stars, Title: "Математика " + s.Settings.Operation}
}
