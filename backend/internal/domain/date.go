package domain

import "time"

// FamilyDate is the calendar shared by progress and daily reward budgets.
func FamilyDate(now time.Time) string {
	return now.In(time.FixedZone("Europe/Minsk", 3*60*60)).Format("2006-01-02")
}
