// Operator manages the pilot registry through stdin, never command-line secrets.
// Оператор управляет реестром пилота через stdin, без секретов в аргументах.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lobov/familyquest/backend/internal/store"
	"io"
	"os"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: operator migrate|provision|claim-legacy|registry|payment|refund|suspend|resume|reset-account|grant-runtime < input.json")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, e := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		return e
	}
	defer db.Close()
	p := store.NewPlatform(db)
	cmd := os.Args[1]
	if cmd == "migrate" {
		if e = p.Migrate(ctx); e != nil {
			return e
		}
		return p.MigrateFamilies(ctx)
	}
	decode := func(v any) error {
		d := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20))
		d.DisallowUnknownFields()
		if err := d.Decode(v); err != nil {
			return err
		}
		if err := d.Decode(new(any)); err != io.EOF {
			return fmt.Errorf("expected exactly one JSON document")
		}
		return nil
	}
	switch cmd {
	case "provision", "claim-legacy":
		var in struct{ Name, Email, Password, Parent, PIN, Operator string }
		if e = decode(&in); e != nil {
			return e
		}
		id, e := p.Provision(ctx, in.Name, in.Email, in.Password, in.Parent, in.PIN, in.Operator, cmd == "claim-legacy")
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]int64{"familyId": id})
	case "registry":
		var in struct{ Offset int }
		if e = decode(&in); e != nil {
			return e
		}
		if in.Offset < 0 {
			return fmt.Errorf("offset must be nonnegative")
		}
		v, e := p.Registry(ctx, in.Offset)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(v)
	case "grant-runtime":
		var in struct{ Role string }
		if e = decode(&in); e != nil {
			return e
		}
		return p.ConfigureRuntime(ctx, in.Role)
	case "reset-account":
		var in struct{ Email, Password, Operator string }
		if e = decode(&in); e != nil {
			return e
		}
		return p.ResetAccount(ctx, in.Email, in.Password, in.Operator)
	case "refund":
		var in struct{ Reference, Operator string }
		if e = decode(&in); e != nil {
			return e
		}
		return p.RefundPayment(ctx, in.Reference, in.Operator)
	case "payment":
		var in store.ManualPayment
		if e = decode(&in); e != nil {
			return e
		}
		return p.RecordPayment(ctx, in)
	case "suspend", "resume":
		var in struct {
			FamilyID int64
			Operator string
		}
		if e = decode(&in); e != nil {
			return e
		}
		return p.Suspend(ctx, in.FamilyID, cmd == "suspend", in.Operator)
	}
	return fmt.Errorf("unknown command")
}
