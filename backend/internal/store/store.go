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

		insert into participants (name, role, pin_code) values
			('Мама', 'parent', '111111'),
			('Папа', 'parent', '222222'),
			('Макс', 'child', '333333')
		on conflict (name) do update set role = excluded.role, pin_code = excluded.pin_code;

		insert into chores (title, description, schedule, time_window, benefit_type, execution_mode, base_value) values
			('Почистить зубы', 'Макс сам чистит зубы утром. Родитель только помогает с таймером и проверяет улыбку.', 'daily', 'morning', 'self', 'assigned', 20),
			('Заправить кровать', 'Поправить подушку, одеяло и любимую игрушку после сна.', 'daily', 'morning', 'self', 'assigned', 25),
			('Убрать игрушки', 'Вернуть игрушки в коробки и освободить пол перед вечерними делами.', 'daily', 'evening', 'self', 'assigned', 35),
			('Помочь накрыть на стол', 'Поставить салфетки, ложки или безопасную посуду для семейного приема пищи.', 'daily', 'evening', 'family', 'adult_child', 30),
			('Полить растение', 'Налить немного воды в одно домашнее растение вместе со взрослым.', 'weekly', 'day', 'home', 'adult_child', 40),
			('Приготовить завтрак', 'Собрать простой семейный завтрак и оставить кухню готовой к дню.', 'daily', 'morning', 'family', 'assigned', 80),
			('Почитать с Максом', 'Спокойное чтение, пересказ или разговор по книге перед сном.', 'daily', 'evening', 'care', 'together', 60),
			('Запустить стирку', 'Собрать вещи, выбрать режим и развесить или переложить белье.', 'weekly', 'day', 'family', 'assigned', 90),
			('Вынести мусор', 'Проверить кухню и вынести пакет в контейнер.', 'daily', 'evening', 'family', 'assigned', 45),
			('Закупить продукты', 'Проверить список, купить базовые продукты и разобрать пакеты дома.', 'weekly', 'day', 'family', 'anyone', 120),
			('Оплатить семейные счета', 'Проверить регулярные платежи и отметить важные расходы месяца.', 'monthly', 'day', 'family', 'assigned', 150)
		on conflict (title) do update set
			description = excluded.description,
			schedule = excluded.schedule,
			time_window = excluded.time_window,
			benefit_type = excluded.benefit_type,
			execution_mode = excluded.execution_mode,
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

func (s *Store) VerifyParticipantPIN(ctx context.Context, participantID int64, pin string) (Participant, error) {
	var participant Participant
	err := s.pool.QueryRow(ctx, `
		select id, name, role, created_at
		from participants
		where id = $1 and pin_code = $2
	`, participantID, pin).Scan(&participant.ID, &participant.Name, &participant.Role, &participant.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Participant{}, ErrInvalidPIN
	}
	if err != nil {
		return Participant{}, err
	}
	return participant, nil
}

func (s *Store) ListChores(ctx context.Context) ([]Chore, error) {
	rows, err := s.pool.Query(ctx, `select id, title, description, schedule, time_window, benefit_type, execution_mode, base_value, active, created_at from chores where active = true order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chores []Chore
	for rows.Next() {
		var c Chore
		if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.Schedule, &c.TimeWindow, &c.BenefitType, &c.ExecutionMode, &c.BaseValue, &c.Active, &c.CreatedAt); err != nil {
			return nil, err
		}
		chores = append(chores, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(chores) == 0 {
		return chores, nil
	}
	if err := s.loadChoreParticipants(ctx, chores); err != nil {
		return nil, err
	}
	return chores, nil
}

func (s *Store) CreateChore(ctx context.Context, chore Chore) (Chore, error) {
	if chore.BenefitType == "" {
		chore.BenefitType = "self"
	}
	if chore.ExecutionMode == "" {
		chore.ExecutionMode = "assigned"
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return chore, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		insert into chores (title, description, schedule, time_window, benefit_type, execution_mode, base_value)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning id, title, description, schedule, time_window, benefit_type, execution_mode, base_value, active, created_at
	`, chore.Title, chore.Description, chore.Schedule, chore.TimeWindow, chore.BenefitType, chore.ExecutionMode, chore.BaseValue).
		Scan(&chore.ID, &chore.Title, &chore.Description, &chore.Schedule, &chore.TimeWindow, &chore.BenefitType, &chore.ExecutionMode, &chore.BaseValue, &chore.Active, &chore.CreatedAt)
	if err != nil {
		return chore, err
	}
	if err := syncChoreParticipants(ctx, tx, chore.ID, chore.ParticipantIDs); err != nil {
		return chore, err
	}
	if err := tx.Commit(ctx); err != nil {
		return chore, err
	}
	chores, err := s.ListChores(ctx)
	if err != nil {
		return chore, err
	}
	for _, item := range chores {
		if item.ID == chore.ID {
			return item, nil
		}
	}
	return chore, nil
}

func (s *Store) UpdateChore(ctx context.Context, chore Chore) (Chore, error) {
	if chore.BenefitType == "" {
		chore.BenefitType = "self"
	}
	if chore.ExecutionMode == "" {
		chore.ExecutionMode = "assigned"
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return chore, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		update chores
		set title = $2,
		    description = $3,
		    schedule = $4,
		    time_window = $5,
		    benefit_type = $6,
		    execution_mode = $7,
		    base_value = $8,
		    active = true
		where id = $1
		returning id, title, description, schedule, time_window, benefit_type, execution_mode, base_value, active, created_at
	`, chore.ID, chore.Title, chore.Description, chore.Schedule, chore.TimeWindow, chore.BenefitType, chore.ExecutionMode, chore.BaseValue).
		Scan(&chore.ID, &chore.Title, &chore.Description, &chore.Schedule, &chore.TimeWindow, &chore.BenefitType, &chore.ExecutionMode, &chore.BaseValue, &chore.Active, &chore.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Chore{}, ErrNotFound
	}
	if err != nil {
		return chore, err
	}
	if err := syncChoreParticipants(ctx, tx, chore.ID, chore.ParticipantIDs); err != nil {
		return chore, err
	}
	if err := tx.Commit(ctx); err != nil {
		return chore, err
	}
	chores, err := s.ListChores(ctx)
	if err != nil {
		return chore, err
	}
	for _, item := range chores {
		if item.ID == chore.ID {
			return item, nil
		}
	}
	return chore, nil
}

func (s *Store) ListAssignments(ctx context.Context) ([]Assignment, error) {
	rows, err := s.pool.Query(ctx, `
		select a.id, a.chore_id, a.participant_id, c.title, p.name, c.schedule, c.time_window, c.benefit_type, c.execution_mode, c.base_value, a.created_at
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
		if err := rows.Scan(&a.ID, &a.ChoreID, &a.ParticipantID, &a.ChoreTitle, &a.PersonName, &a.Schedule, &a.TimeWindow, &a.BenefitType, &a.ExecutionMode, &a.BaseValue, &a.CreatedAt); err != nil {
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

func (s *Store) loadChoreParticipants(ctx context.Context, chores []Chore) error {
	choreByID := make(map[int64]*Chore, len(chores))
	ids := make([]int64, 0, len(chores))
	for index := range chores {
		choreByID[chores[index].ID] = &chores[index]
		ids = append(ids, chores[index].ID)
	}

	rows, err := s.pool.Query(ctx, `
		select a.chore_id, p.id, p.name
		from assignments a
		join participants p on p.id = a.participant_id
		where a.active = true and a.chore_id = any($1)
		order by a.chore_id, p.id
	`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var choreID int64
		var participantID int64
		var name string
		if err := rows.Scan(&choreID, &participantID, &name); err != nil {
			return err
		}
		chore := choreByID[choreID]
		if chore == nil {
			continue
		}
		chore.ParticipantIDs = append(chore.ParticipantIDs, participantID)
		chore.ParticipantNames = append(chore.ParticipantNames, name)
	}
	return rows.Err()
}

func syncChoreParticipants(ctx context.Context, tx pgx.Tx, choreID int64, participantIDs []int64) error {
	_, err := tx.Exec(ctx, `update assignments set active = false where chore_id = $1`, choreID)
	if err != nil {
		return err
	}
	for _, participantID := range participantIDs {
		if participantID == 0 {
			continue
		}
		_, err := tx.Exec(ctx, `
			insert into assignments (chore_id, participant_id)
			values ($1, $2)
			on conflict (chore_id, participant_id) do update set active = true
		`, choreID, participantID)
		if err != nil {
			return err
		}
	}
	return nil
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
			c.schedule = 'daily'
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
		       c.time_window, c.benefit_type, c.execution_mode, t.status, t.completed_at, t.confirmed_at,
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
		group by t.id, a.chore_id, a.participant_id, c.title, p.name, c.time_window, c.benefit_type, c.execution_mode, c.base_value
		order by a.participant_id, c.time_window, c.title
	`, dueDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.AssignmentID, &t.ChoreID, &t.ParticipantID, &t.ChoreTitle, &t.PersonName, &t.DueDate, &t.TimeWindow, &t.BenefitType, &t.ExecutionMode, &t.Status, &t.CompletedAt, &t.ConfirmedAt, &t.AverageRating, &t.Reward); err != nil {
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

func (s *Store) RateBehavior(ctx context.Context, ratedDate time.Time, raterID, targetID int64, rating int, comment string) (BehaviorRating, error) {
	if rating < 1 || rating > 5 {
		return BehaviorRating{}, ErrInvalidRating
	}
	var behavior BehaviorRating
	err := s.pool.QueryRow(ctx, `
		insert into behavior_ratings (rated_date, rater_participant_id, target_participant_id, rating, comment)
		values ($1::date, $2, $3, $4, $5)
		on conflict (rated_date, rater_participant_id, target_participant_id)
		do update set rating = excluded.rating, comment = excluded.comment, created_at = now()
		returning id, rated_date::text, rater_participant_id, target_participant_id, rating, comment, created_at
	`, ratedDate.Format("2006-01-02"), raterID, targetID, rating, comment).
		Scan(&behavior.ID, &behavior.RatedDate, &behavior.RaterParticipantID, &behavior.TargetParticipantID, &behavior.Rating, &behavior.Comment, &behavior.CreatedAt)
	return behavior, err
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
		select p.id, p.name,
		       coalesce(activity.tasks_done, 0)::int as tasks_done,
		       coalesce(planned.tasks_assigned, 0)::int as tasks_assigned,
		       coalesce(activity.reward, 0)::float as reward,
		       coalesce(activity.average_rating, 0)::float as average_rating,
		       coalesce(behavior.behavior_rating, 0)::float as behavior_rating,
		       coalesce(behavior.behavior_count, 0)::int as behavior_count
		from participants p
		left join lateral (
			select count(*)::int as tasks_assigned
			from assignments a
			join chores c on c.id = a.chore_id and c.active = true
			cross join generate_series($1::date, ($2::date - interval '1 day')::date, interval '1 day') as days(day)
			where a.participant_id = p.id
			  and a.active = true
			  and (
				c.schedule = 'daily'
				or (c.schedule = 'weekly' and extract(isodow from days.day) = 6)
				or (c.schedule = 'monthly' and extract(day from days.day) = 1)
			  )
		) planned on true
		left join lateral (
			select count(t.id) filter (where t.status <> 'pending')::int as tasks_done,
			       coalesce(round(sum(c.base_value * ratings.avg_rating / 5.0) filter (where t.status = 'confirmed')::numeric, 2), 0)::float as reward,
			       coalesce(round(avg(ratings.avg_rating) filter (where t.status = 'confirmed')::numeric, 2), 0)::float as average_rating
			from assignments a
			join chores c on c.id = a.chore_id and c.active = true
			join tasks t on t.assignment_id = a.id and t.due_date >= $1::date and t.due_date < $2::date
			left join lateral (
				select avg(rating)::float as avg_rating
				from confirmations
				where task_id = t.id
			) ratings on true
			where a.participant_id = p.id
			  and a.active = true
		) activity on true
		left join lateral (
			select round(avg(rating)::numeric, 2)::float as behavior_rating,
			       count(*)::int as behavior_count
			from behavior_ratings
			where target_participant_id = p.id
			  and rated_date >= $1::date
			  and rated_date < $2::date
		) behavior on true
		order by reward desc, behavior_rating desc, tasks_done desc, tasks_assigned desc, p.id
	`, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.ParticipantID, &entry.Name, &entry.TasksDone, &entry.TasksAssigned, &entry.Reward, &entry.AverageRating, &entry.BehaviorRating, &entry.BehaviorCount); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func periodBounds(period string, at time.Time) (time.Time, time.Time) {
	year, month, day := at.Date()
	location := at.Location()
	if period == "day" {
		start := time.Date(year, month, day, 0, 0, 0, 0, location)
		return start, start.AddDate(0, 0, 1)
	}
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
	ErrInvalidPIN    = errors.New("invalid pin")
)
