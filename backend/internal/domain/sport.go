package domain

import (
	"math"
	"strings"
)

type SportSession struct {
	Activity   string          `json:"activity"`
	Minutes    float64         `json:"minutes"`
	DistanceKm float64         `json:"distanceKm"`
	Exercises  []SportExercise `json:"exercises"`
}
type SportExercise struct {
	Name     string  `json:"name"`
	Sets     int     `json:"sets"`
	Reps     int     `json:"reps"`
	WeightKg float64 `json:"weightKg"`
}

func (e FamilyEntry) validateSport() error {
	if e.Kind != "sport" {
		if e.Sport != nil {
			return familyInvalid("спортивные показатели доступны только в спорте")
		}
		return nil
	}
	s := e.Sport
	validNumber := func(v, max float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= max }
	if s == nil || len(e.ParticipantIDs) != 1 || !ValidFamilyDate(e.Date) || strings.TrimSpace(s.Activity) == "" || len([]rune(s.Activity)) > 80 {
		return familyInvalid("укажите участника, дату и вид спорта")
	}
	if !validNumber(s.Minutes, 1440) || s.Minutes == 0 || !validNumber(s.DistanceKm, 2000) || len(s.Exercises) > 50 {
		return familyInvalid("проверьте длительность, дистанцию и упражнения")
	}
	for _, x := range s.Exercises {
		if strings.TrimSpace(x.Name) == "" || len([]rune(x.Name)) > 100 || x.Sets < 1 || x.Sets > 100 || x.Reps < 1 || x.Reps > 10000 || !validNumber(x.WeightKg, 1000) {
			return familyInvalid("проверьте название, подходы, повторения и вес упражнения")
		}
	}
	return nil
}
