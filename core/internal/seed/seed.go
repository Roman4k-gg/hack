package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Options struct {
	Demo bool
}

const (
	teamSearch   = "11111111-1111-1111-1111-111111111111"
	teamAds      = "11111111-1111-1111-1111-111111111112"
	teamPlatform = "11111111-1111-1111-1111-111111111113"

	userPetr  = "22222222-2222-2222-2222-222222222221"
	userAdmin = "22222222-2222-2222-2222-222222222222"
	userMasha = "22222222-2222-2222-2222-222222222223"
	userDasha = "22222222-2222-2222-2222-222222222224"

	planDasha = "33333333-3333-3333-3333-333333333331"

	stageEnvironment = "33333333-3333-3333-3333-333333333341"
	stageTeam        = "33333333-3333-3333-3333-333333333342"
	stageProcesses   = "33333333-3333-3333-3333-333333333343"
	stageFirstTasks  = "33333333-3333-3333-3333-333333333344"
)

type team struct {
	id   string
	slug string
	name string
}

type user struct {
	id       string
	email    string
	fullName string
	role     string
	teamID   string
	mentorID string
	position string
}

type stage struct {
	id          string
	title       string
	description string
	position    int
}

type item struct {
	id            string
	stageID       string
	title         string
	description   string
	kind          string
	status        string
	blockedReason string
	position      int
	dueInDays     int
}

type document struct {
	id      string
	slug    string
	title   string
	summary string
	content string
	url     string
	teamID  string
	tags    []string
}

func Run(ctx context.Context, pool *pgxpool.Pool, opts Options) error {
	if pool == nil {
		return fmt.Errorf("seed: postgres pool is not initialised")
	}
	if !opts.Demo {
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("seed: begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := seedTeams(ctx, tx); err != nil {
		return err
	}
	if err := seedUsers(ctx, tx); err != nil {
		return err
	}
	if err := seedOnboarding(ctx, tx); err != nil {
		return err
	}
	if err := seedKnowledgeBase(ctx, tx); err != nil {
		return err
	}
	if err := seedActivity(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("seed: commit: %w", err)
	}
	return nil
}

func seedTeams(ctx context.Context, tx pgx.Tx) error {
	teams := []team{
		{id: teamSearch, slug: "search", name: "Search"},
		{id: teamAds, slug: "ads", name: "Ads"},
		{id: teamPlatform, slug: "platform", name: "Platform"},
	}

	for _, t := range teams {
		_, err := tx.Exec(ctx, `
			INSERT INTO teams (id, slug, name)
			VALUES ($1, $2, $3)
			ON CONFLICT (id) DO UPDATE
			SET slug = EXCLUDED.slug,
			    name = EXCLUDED.name`,
			t.id, t.slug, t.name,
		)
		if err != nil {
			return fmt.Errorf("seed: upsert team %s: %w", t.slug, err)
		}
	}
	return nil
}

func seedUsers(ctx context.Context, tx pgx.Tx) error {
	users := []user{
		{
			id: userPetr, email: "petr@demo.local", fullName: "Пётр Иванов",
			role: "mentor", teamID: teamSearch, position: "Ведущий разработчик",
		},
		{
			id: userAdmin, email: "admin@demo.local", fullName: "Администратор",
			role: "admin", teamID: teamPlatform, position: "Platform team",
		},
		{
			id: userMasha, email: "masha@demo.local", fullName: "Мария Кузнецова",
			role: "intern", teamID: teamAds, position: "Стажёр-разработчик",
		},
		{
			id: userDasha, email: "dasha@demo.local", fullName: "Дарья Соколова",
			role: "intern", teamID: teamSearch, mentorID: userPetr,
			position: "Стажёр-разработчик",
		},
	}

	for _, u := range users {
		var mentor any
		if u.mentorID != "" {
			mentor = u.mentorID
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, email, full_name, role, team_id, mentor_id, position)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE
			SET email = EXCLUDED.email,
			    full_name = EXCLUDED.full_name,
			    role = EXCLUDED.role,
			    team_id = EXCLUDED.team_id,
			    mentor_id = EXCLUDED.mentor_id,
			    position = EXCLUDED.position`,
			u.id, u.email, u.fullName, u.role, u.teamID, mentor, u.position,
		)
		if err != nil {
			return fmt.Errorf("seed: upsert user %s: %w", u.email, err)
		}
	}
	return nil
}

func seedOnboarding(ctx context.Context, tx pgx.Tx) error {
	startsOn := time.Now().AddDate(0, 0, -3)

	_, err := tx.Exec(ctx, `
		INSERT INTO onboarding_plans (id, user_id, title, starts_on)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    title = EXCLUDED.title,
		    starts_on = EXCLUDED.starts_on`,
		planDasha, userDasha, "Адаптация в команде Search", startsOn,
	)
	if err != nil {
		return fmt.Errorf("seed: upsert onboarding plan: %w", err)
	}

	stages := []stage{
		{id: stageEnvironment, title: "Неделя 1 · Окружение", description: "Доступы, ноутбук и рабочее окружение.", position: 1},
		{id: stageTeam, title: "Неделя 1 · Команда", description: "Знакомство с командой и наставником.", position: 2},
		{id: stageProcesses, title: "Неделя 2 · Процессы", description: "Как устроены разработка, ревью и релизы.", position: 3},
		{id: stageFirstTasks, title: "Месяц 1 · Первые задачи", description: "Первый вклад в продукт.", position: 4},
	}

	for _, s := range stages {
		_, err := tx.Exec(ctx, `
			INSERT INTO onboarding_stages (id, plan_id, title, description, position)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE
			SET title = EXCLUDED.title,
			    description = EXCLUDED.description,
			    position = EXCLUDED.position`,
			s.id, planDasha, s.title, s.description, s.position,
		)
		if err != nil {
			return fmt.Errorf("seed: upsert onboarding stage %s: %w", s.id, err)
		}
	}

	items := []item{
		{
			id: "44444444-4444-4444-4444-444444444401", stageID: stageEnvironment,
			title: "Получить ноутбук и доступ к VPN", description: "Выдаёт IT-отдел, доступ приходит на рабочую почту.",
			kind: "access", status: "done", position: 1, dueInDays: -2,
		},
		{
			id: "44444444-4444-4444-4444-444444444402", stageID: stageEnvironment,
			title: "Настроить рабочее окружение", description: "Go, Docker, редактор и доступы к репозиторию.",
			kind: "task", status: "done", position: 2, dueInDays: -1,
		},
		{
			id: "44444444-4444-4444-4444-444444444403", stageID: stageEnvironment,
			title: "Получить доступ к Jira", description: "Нужен для работы с задачами команды Search.",
			kind: "access", status: "in_progress", position: 3, dueInDays: 1,
		},
		{
			id: "44444444-4444-4444-4444-444444444404", stageID: stageTeam,
			title: "Познакомиться с командой", description: "Представиться в командном чате и на дейлике.",
			kind: "meeting", status: "done", position: 1, dueInDays: -1,
		},
		{
			id: "44444444-4444-4444-4444-444444444405", stageID: stageTeam,
			title: "Встреча 1:1 с наставником", description: "Обсудить ожидания, цели на месяц и формат поддержки.",
			kind: "meeting", status: "done", position: 2, dueInDays: 0,
		},
		{
			id: "44444444-4444-4444-4444-444444444406", stageID: stageTeam,
			title: "Прочитать гайд по код-ревью", description: "Правила ревью, размер PR и работа с комментариями.",
			kind: "doc", status: "pending", position: 3, dueInDays: 2,
		},
		{
			id: "44444444-4444-4444-4444-444444444407", stageID: stageProcesses,
			title: "Пройти курс по безопасности", description: "Обязательный курс для всех новых сотрудников.",
			kind: "task", status: "blocked", blockedReason: "Ждём подтверждения от HR",
			position: 1, dueInDays: 3,
		},
		{
			id: "44444444-4444-4444-4444-444444444408", stageID: stageProcesses,
			title: "Разобраться с процессом релизов", description: "Как устроены ветки, стенды и деплой.",
			kind: "doc", status: "pending", position: 2, dueInDays: 4,
		},
		{
			id: "44444444-4444-4444-4444-444444444409", stageID: stageProcesses,
			title: "Сделать первый PR", description: "Небольшая правка с ревью наставника.",
			kind: "task", status: "in_progress", position: 3, dueInDays: 5,
		},
		{
			id: "44444444-4444-4444-4444-444444444410", stageID: stageFirstTasks,
			title: "Взять первую задачу из бэклога", description: "Задача с меткой good first issue в команде Search.",
			kind: "task", status: "pending", position: 1, dueInDays: 10,
		},
	}

	for _, it := range items {
		var due any
		if it.dueInDays != 0 {
			due = time.Now().AddDate(0, 0, it.dueInDays)
		}
		var blocked any
		if it.blockedReason != "" {
			blocked = it.blockedReason
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO onboarding_items
				(id, plan_id, stage_id, title, description, kind, status, blocked_reason, due_on, position)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (id) DO UPDATE
			SET stage_id = EXCLUDED.stage_id,
			    title = EXCLUDED.title,
			    description = EXCLUDED.description,
			    kind = EXCLUDED.kind,
			    status = EXCLUDED.status,
			    blocked_reason = EXCLUDED.blocked_reason,
			    due_on = EXCLUDED.due_on,
			    position = EXCLUDED.position,
			    completed_at = CASE
			        WHEN EXCLUDED.status = 'done' THEN COALESCE(onboarding_items.completed_at, now())
			        ELSE NULL
			    END`,
			it.id, planDasha, it.stageID, it.title, it.description,
			it.kind, it.status, blocked, due, it.position,
		)
		if err != nil {
			return fmt.Errorf("seed: upsert onboarding item %s: %w", it.id, err)
		}
	}

	return nil
}

func seedKnowledgeBase(ctx context.Context, tx pgx.Tx) error {
	documents := []document{
		{
			id: "55555555-5555-5555-5555-555555555501", slug: "jira-access",
			title:   "Как получить доступ к Jira",
			summary: "Пошаговая инструкция для стажёров команды Search.",
			content: "1. Напишите наставнику в чат.\n2. Наставник создаёт заявку в IT-портале.\n3. Доступ приходит на рабочую почту в течение дня.\n4. Войдите в Jira через SSO.",
			url:     "https://demo.local/docs/jira-access", teamID: teamSearch,
			tags: []string{"jira", "доступ", "access", "search"},
		},
		{
			id: "55555555-5555-5555-5555-555555555502", slug: "vpn-setup",
			title:   "Настройка VPN",
			summary: "Как подключиться к корпоративной сети с ноутбука.",
			content: "Установите клиент из IT-портала, импортируйте профиль и войдите под рабочей учётной записью. При проблемах пишите в #it-help.",
			url:     "https://demo.local/docs/vpn-setup", teamID: teamPlatform,
			tags: []string{"vpn", "сеть", "it"},
		},
		{
			id: "55555555-5555-5555-5555-555555555503", slug: "code-review-guide",
			title:   "Гайд по код-ревью",
			summary: "Правила ревью и ожидания от автора PR.",
			content: "Держите PR небольшим (до 400 строк), описывайте изменения и добавляйте тесты. Ревьюер отвечает в течение рабочего дня.",
			url:     "https://demo.local/docs/code-review", teamID: teamSearch,
			tags: []string{"ревью", "pr", "процессы"},
		},
		{
			id: "55555555-5555-5555-5555-555555555504", slug: "release-process",
			title:   "Процесс релизов",
			summary: "Ветки, стенды и деплой в прод.",
			content: "Разработка идёт в main, релиз — по расписанию два раза в неделю. Перед деплоем прогоняются тесты и проверка на стенде stage.",
			url:     "https://demo.local/docs/release-process", teamID: teamPlatform,
			tags: []string{"релиз", "деплой", "процессы"},
		},
		{
			id: "55555555-5555-5555-5555-555555555505", slug: "vacation-policy",
			title:   "Отпуск и отгулы",
			summary: "Сколько дней доступно и как их согласовать.",
			content: "Стажёру доступны оплачиваемые дни отпуска согласно договору. Заявка создаётся в HR-системе и согласуется с наставником за две недели.",
			url:     "https://demo.local/docs/vacation", teamID: "",
			tags: []string{"отпуск", "hr", "отгул"},
		},
	}

	for _, d := range documents {
		var teamID any
		if d.teamID != "" {
			teamID = d.teamID
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO kb_documents (id, slug, title, summary, content, url, team_id, tags)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO UPDATE
			SET slug = EXCLUDED.slug,
			    title = EXCLUDED.title,
			    summary = EXCLUDED.summary,
			    content = EXCLUDED.content,
			    url = EXCLUDED.url,
			    team_id = EXCLUDED.team_id,
			    tags = EXCLUDED.tags`,
			d.id, d.slug, d.title, d.summary, d.content, d.url, teamID, d.tags,
		)
		if err != nil {
			return fmt.Errorf("seed: upsert kb document %s: %w", d.slug, err)
		}
	}

	return nil
}

func seedActivity(ctx context.Context, tx pgx.Tx) error {
	audit := []struct {
		id         string
		action     string
		entityType string
		entityID   string
		payload    string
	}{
		{
			id: "66666666-6666-6666-6666-666666666601", action: "onboarding.plan_created",
			entityType: "onboarding_plan", entityID: planDasha,
			payload: `{"source":"seed"}`,
		},
		{
			id: "66666666-6666-6666-6666-666666666602", action: "onboarding.item_completed",
			entityType: "onboarding_item", entityID: "44444444-4444-4444-4444-444444444401",
			payload: `{"source":"seed","status":"done"}`,
		},
	}

	for _, a := range audit {
		_, err := tx.Exec(ctx, `
			INSERT INTO audit_log (id, actor_id, action, entity_type, entity_id, payload, trace_id)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
			ON CONFLICT (id) DO NOTHING`,
			a.id, userDasha, a.action, a.entityType, a.entityID, a.payload, "seed",
		)
		if err != nil {
			return fmt.Errorf("seed: insert audit log %s: %w", a.action, err)
		}
	}

	events := []struct {
		id         string
		name       string
		entityType string
		entityID   string
		payload    string
	}{
		{
			id: "77777777-7777-7777-7777-777777777701", name: "onboarding_plan_created",
			entityType: "onboarding_plan", entityID: planDasha,
			payload: `{"source":"seed"}`,
		},
		{
			id: "77777777-7777-7777-7777-777777777702", name: "onboarding_item_completed",
			entityType: "onboarding_item", entityID: "44444444-4444-4444-4444-444444444401",
			payload: `{"source":"seed"}`,
		},
		{
			id: "77777777-7777-7777-7777-777777777703", name: "onboarding_item_completed",
			entityType: "onboarding_item", entityID: "44444444-4444-4444-4444-444444444402",
			payload: `{"source":"seed"}`,
		},
	}

	for _, e := range events {
		_, err := tx.Exec(ctx, `
			INSERT INTO events (id, user_id, name, entity_type, entity_id, payload, trace_id)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
			ON CONFLICT (id) DO NOTHING`,
			e.id, userDasha, e.name, e.entityType, e.entityID, e.payload, "seed",
		)
		if err != nil {
			return fmt.Errorf("seed: insert event %s: %w", e.name, err)
		}
	}

	return nil
}
