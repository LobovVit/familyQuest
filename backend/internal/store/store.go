package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := os.ReadDir("migrations")
	if err != nil {
		if entries, err = os.ReadDir(filepath.Join("backend", "migrations")); err != nil {
			return err
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		path := filepath.Join("migrations", entry.Name())
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join("backend", "migrations", entry.Name())
		}
		sql, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := s.pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("%s: %w", entry.Name(), err)
		}
	}
	return nil
}

func (s *Store) Seed(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		update participants
		set name = 'Макс'
		where name = 'Сын'
		  and not exists (select 1 from participants where name = 'Макс');

		insert into participants (name, role) values
			('Мама', 'parent'),
			('Папа', 'parent'),
			('Макс', 'child')
		on conflict (name) do update set role = excluded.role;

		insert into chores (title, description, schedule, time_window, base_value) values
			('Почистить зубы', 'Макс сам чистит зубы утром. Родитель только помогает с таймером и проверяет улыбку.', 'daily_windowed', 'morning', 20),
			('Заправить кровать', 'Поправить подушку, одеяло и любимую игрушку после сна.', 'daily_windowed', 'morning', 25),
			('Убрать игрушки', 'Вернуть игрушки в коробки и освободить пол перед вечерними делами.', 'daily_windowed', 'evening', 35),
			('Помочь накрыть на стол', 'Поставить салфетки, ложки или безопасную посуду для семейного приема пищи.', 'daily_windowed', 'evening', 30),
			('Полить растение', 'Налить немного воды в одно домашнее растение вместе со взрослым.', 'weekly', 'day', 40),
			('Приготовить завтрак', 'Собрать простой семейный завтрак и оставить кухню готовой к дню.', 'daily_windowed', 'morning', 80),
			('Почитать с Максом', 'Спокойное чтение, пересказ или разговор по книге перед сном.', 'daily_windowed', 'evening', 60),
			('Запустить стирку', 'Собрать вещи, выбрать режим и развесить или переложить белье.', 'weekly', 'day', 90),
			('Вынести мусор', 'Проверить кухню и вынести пакет в контейнер.', 'daily_windowed', 'evening', 45),
			('Закупить продукты', 'Проверить список, купить базовые продукты и разобрать пакеты дома.', 'weekly', 'day', 120),
			('Оплатить семейные счета', 'Проверить регулярные платежи и отметить важные расходы месяца.', 'monthly', 'day', 150)
		on conflict (title) do update set
			description = excluded.description,
			schedule = excluded.schedule,
			time_window = excluded.time_window,
			base_value = excluded.base_value,
			active = true;

		update assignments
		set active = false
		from chores c
		where assignments.chore_id = c.id
		  and c.title in ('Убрать комнату', 'Помочь с завтраком', 'Вымыть полы', 'Разобрать школьный рюкзак');

		update chores
		set active = false
		where title in ('Убрать комнату', 'Помочь с завтраком', 'Вымыть полы', 'Разобрать школьный рюкзак');

		insert into assignments (chore_id, participant_id)
		select c.id, p.id
		from chores c
		join participants p on (
			(p.name = 'Макс' and c.title in ('Почистить зубы', 'Заправить кровать', 'Убрать игрушки', 'Помочь накрыть на стол', 'Полить растение'))
			or (p.name = 'Мама' and c.title in ('Приготовить завтрак', 'Почитать с Максом', 'Запустить стирку'))
			or (p.name = 'Папа' and c.title in ('Вынести мусор', 'Закупить продукты', 'Оплатить семейные счета'))
		)
		on conflict (chore_id, participant_id) do update set active = true;
	`)
	return err
}

func (s *Store) ListParticipants(ctx context.Context) ([]Participant, error) {
	rows, err := s.pool.Query(ctx, `select id, name, role, created_at from participants order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []Participant
	for rows.Next() {
		var p Participant
		if err := rows.Scan(&p.ID, &p.Name, &p.Role, &p.CreatedAt); err != nil {
			return nil, err
		}
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

func (s *Store) ListChores(ctx context.Context) ([]Chore, error) {
	rows, err := s.pool.Query(ctx, `select id, title, description, schedule, time_window, base_value, active, created_at from chores where active = true order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chores []Chore
	for rows.Next() {
		var c Chore
		if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.Schedule, &c.TimeWindow, &c.BaseValue, &c.Active, &c.CreatedAt); err != nil {
			return nil, err
		}
		chores = append(chores, c)
	}
	return chores, rows.Err()
}

func (s *Store) CreateChore(ctx context.Context, chore Chore) (Chore, error) {
	err := s.pool.QueryRow(ctx, `
		insert into chores (title, description, schedule, time_window, base_value)
		values ($1, $2, $3, $4, $5)
		returning id, title, description, schedule, time_window, base_value, active, created_at
	`, chore.Title, chore.Description, chore.Schedule, chore.TimeWindow, chore.BaseValue).
		Scan(&chore.ID, &chore.Title, &chore.Description, &chore.Schedule, &chore.TimeWindow, &chore.BaseValue, &chore.Active, &chore.CreatedAt)
	return chore, err
}

func (s *Store) ListAssignments(ctx context.Context) ([]Assignment, error) {
	rows, err := s.pool.Query(ctx, `
		select a.id, a.chore_id, a.participant_id, c.title, p.name, c.schedule, c.time_window, c.base_value, a.created_at
		from assignments a
		join chores c on c.id = a.chore_id
		join participants p on p.id = a.participant_id
		where a.active = true and c.active = true
		order by a.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []Assignment
	for rows.Next() {
		var a Assignment
		if err := rows.Scan(&a.ID, &a.ChoreID, &a.ParticipantID, &a.ChoreTitle, &a.PersonName, &a.Schedule, &a.TimeWindow, &a.BaseValue, &a.CreatedAt); err != nil {
			return nil, err
		}
		assignments = append(assignments, a)
	}
	return assignments, rows.Err()
}

func (s *Store) CreateAssignment(ctx context.Context, choreID, participantID int64) (Assignment, error) {
	var assignment Assignment
	err := s.pool.QueryRow(ctx, `
		insert into assignments (chore_id, participant_id)
		values ($1, $2)
		on conflict (chore_id, participant_id) do update set active = true
		returning id, chore_id, participant_id, created_at
	`, choreID, participantID).Scan(&assignment.ID, &assignment.ChoreID, &assignment.ParticipantID, &assignment.CreatedAt)
	if err != nil {
		return assignment, err
	}
	assignments, err := s.ListAssignments(ctx)
	if err != nil {
		return assignment, err
	}
	for _, item := range assignments {
		if item.ID == assignment.ID {
			return item, nil
		}
	}
	return assignment, nil
}

func (s *Store) EnsureTasksForDate(ctx context.Context, dueDate time.Time) error {
	_, err := s.pool.Exec(ctx, `
		insert into tasks (assignment_id, due_date)
		select a.id, $1::date
		from assignments a
		join chores c on c.id = a.chore_id
		where a.active = true
		  and c.active = true
		  and (
			c.schedule in ('daily', 'daily_windowed')
			or (c.schedule = 'weekly' and extract(isodow from $1::date) = 6)
			or (c.schedule = 'monthly' and extract(day from $1::date) = 1)
		  )
		on conflict (assignment_id, due_date) do nothing
	`, dueDate.Format("2006-01-02"))
	return err
}

func (s *Store) ListTasks(ctx context.Context, dueDate time.Time) ([]Task, error) {
	if err := s.EnsureTasksForDate(ctx, dueDate); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		select t.id, t.assignment_id, a.chore_id, a.participant_id, c.title, p.name, t.due_date::text,
		       c.time_window, t.status, t.completed_at, t.confirmed_at,
		       coalesce(avg(conf.rating), 0)::float,
		       coalesce(round((c.base_value * coalesce(avg(conf.rating), 0) / 5.0)::numeric, 2), 0)::float
		from tasks t
		join assignments a on a.id = t.assignment_id
		join chores c on c.id = a.chore_id
		join participants p on p.id = a.participant_id
		left join confirmations conf on conf.task_id = t.id
		where t.due_date = $1::date
		  and a.active = true
		  and c.active = true
		group by t.id, a.chore_id, a.participant_id, c.title, p.name, c.time_window, c.base_value
		order by a.participant_id, c.time_window, c.title
	`, dueDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.AssignmentID, &t.ChoreID, &t.ParticipantID, &t.ChoreTitle, &t.PersonName, &t.DueDate, &t.TimeWindow, &t.Status, &t.CompletedAt, &t.ConfirmedAt, &t.AverageRating, &t.Reward); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Store) CompleteTask(ctx context.Context, taskID, participantID int64) (Task, error) {
	var dueDate time.Time
	err := s.pool.QueryRow(ctx, `
		update tasks
		set status = 'completed', completed_by = $2, completed_at = now()
		where id = $1 and status in ('pending', 'needs_work')
		returning due_date
	`, taskID, participantID).Scan(&dueDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	return s.GetTask(ctx, taskID, dueDate)
}

func (s *Store) ConfirmTask(ctx context.Context, taskID, participantID int64, rating int, comment string) (Task, error) {
	if rating < 1 || rating > 5 {
		return Task{}, ErrInvalidRating
	}
	var dueDate time.Time
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `select due_date from tasks where id = $1`, taskID).Scan(&dueDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	_, err = tx.Exec(ctx, `
		insert into confirmations (task_id, participant_id, rating, comment)
		values ($1, $2, $3, $4)
		on conflict (task_id, participant_id)
		do update set rating = excluded.rating, comment = excluded.comment, created_at = now()
	`, taskID, participantID, rating, comment)
	if err != nil {
		return Task{}, err
	}
	_, err = tx.Exec(ctx, `
		update tasks
		set status = 'confirmed', confirmed_at = now()
		where id = $1 and status in ('completed', 'confirmed')
	`, taskID)
	if err != nil {
		return Task{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, err
	}
	return s.GetTask(ctx, taskID, dueDate)
}

func (s *Store) GetTask(ctx context.Context, taskID int64, dueDate time.Time) (Task, error) {
	tasks, err := s.ListTasks(ctx, dueDate)
	if err != nil {
		return Task{}, err
	}
	for _, task := range tasks {
		if task.ID == taskID {
			return task, nil
		}
	}
	return Task{}, ErrNotFound
}

func (s *Store) Leaderboard(ctx context.Context, period string, at time.Time) ([]LeaderboardEntry, error) {
	start, end := periodBounds(period, at)
	rows, err := s.pool.Query(ctx, `
		select p.id, p.name, count(t.id)::int as tasks_done,
		       coalesce(round(sum(c.base_value * ratings.avg_rating / 5.0)::numeric, 2), 0)::float as reward,
		       coalesce(round(avg(ratings.avg_rating)::numeric, 2), 0)::float as average_rating
		from participants p
		left join assignments a on a.participant_id = p.id and a.active = true
		left join tasks t on t.assignment_id = a.id and t.status = 'confirmed' and t.due_date >= $1::date and t.due_date < $2::date
		left join chores c on c.id = a.chore_id and c.active = true
		left join lateral (
			select avg(rating)::float as avg_rating
			from confirmations
			where task_id = t.id
		) ratings on true
		group by p.id, p.name
		order by reward desc, tasks_done desc, p.id
	`, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.ParticipantID, &entry.Name, &entry.TasksDone, &entry.Reward, &entry.AverageRating); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func periodBounds(period string, at time.Time) (time.Time, time.Time) {
	year, month, day := at.Date()
	location := at.Location()
	if period == "month" {
		start := time.Date(year, month, 1, 0, 0, 0, 0, location)
		return start, start.AddDate(0, 1, 0)
	}
	current := time.Date(year, month, day, 0, 0, 0, 0, location)
	weekday := int(current.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := current.AddDate(0, 0, -(weekday - 1))
	return start, start.AddDate(0, 0, 7)
}

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidRating = errors.New("rating must be between 1 and 5")
)
