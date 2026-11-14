package seed

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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

	userPetr   = "22222222-2222-2222-2222-222222222221"
	userAdmin  = "22222222-2222-2222-2222-222222222222"
	userMasha  = "22222222-2222-2222-2222-222222222223"
	userDasha  = "22222222-2222-2222-2222-222222222224"
	userIgor   = "22222222-2222-2222-2222-222222222225"
	userOlga   = "22222222-2222-2222-2222-222222222226"
	userMarina = "22222222-2222-2222-2222-222222222227"
	userTimur  = "22222222-2222-2222-2222-222222222228"
	userVera   = "22222222-2222-2222-2222-222222222229"
	userSonya  = "22222222-2222-2222-2222-222222222230"
	userKirill = "22222222-2222-2222-2222-222222222231"

	planDasha = "33333333-3333-3333-3333-333333333331"

	stageEnvironment = "33333333-3333-3333-3333-333333333341"
	stageTeam        = "33333333-3333-3333-3333-333333333342"
	stageProcesses   = "33333333-3333-3333-3333-333333333343"
	stageFirstTasks  = "33333333-3333-3333-3333-333333333344"
)

var fixedStageIDs = []string{stageEnvironment, stageTeam, stageProcesses, stageFirstTasks}

var stageTemplates = []stageTemplate{
	{title: "Неделя 1 · Окружение", description: "Доступы, ноутбук и рабочее окружение."},
	{title: "Неделя 1 · Команда", description: "Знакомство с командой и наставником."},
	{title: "Неделя 2 · Процессы", description: "Как устроены разработка, ревью и релизы."},
	{title: "Месяц 1 · Первые задачи", description: "Первый вклад в продукт."},
}

var itemTemplates = []itemTemplate{
	{stageIndex: 0, title: "Получить ноутбук и доступ к VPN", description: "Выдаёт IT-отдел, доступ приходит на рабочую почту.", kind: "access", position: 1, dueInDays: -2},
	{stageIndex: 0, title: "Настроить рабочее окружение", description: "Go, Docker, редактор и доступы к репозиторию.", kind: "task", position: 2, dueInDays: -1},
	{stageIndex: 0, title: "Получить доступ к Jira", description: "Нужен для работы с задачами команды.", kind: "access", position: 3, dueInDays: 1},
	{stageIndex: 1, title: "Познакомиться с командой", description: "Представиться в командном чате и на дейлике.", kind: "meeting", position: 1, dueInDays: -1},
	{stageIndex: 1, title: "Встреча 1:1 с наставником", description: "Обсудить ожидания, цели на месяц и формат поддержки.", kind: "meeting", position: 2, dueInDays: 0},
	{stageIndex: 1, title: "Прочитать гайд по код-ревью", description: "Правила ревью, размер PR и работа с комментариями.", kind: "doc", position: 3, dueInDays: 2},
	{stageIndex: 2, title: "Пройти курс по безопасности", description: "Обязательный курс для всех новых сотрудников.", kind: "task", position: 1, dueInDays: 3},
	{stageIndex: 2, title: "Разобраться с процессом релизов", description: "Как устроены ветки, стенды и деплой.", kind: "doc", position: 2, dueInDays: 4},
	{stageIndex: 2, title: "Сделать первый PR", description: "Небольшая правка с ревью наставника.", kind: "task", position: 3, dueInDays: 5},
	{stageIndex: 3, title: "Взять первую задачу из бэклога", description: "Задача с меткой good first issue в команде.", kind: "task", position: 1, dueInDays: 10},
}

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

type stageTemplate struct {
	title       string
	description string
}

type itemTemplate struct {
	stageIndex  int
	title       string
	description string
	kind        string
	position    int
	dueInDays   int
}

type internPlan struct {
	userID       string
	planID       string
	title        string
	startsInDays int
	fixedIDs     bool
	statuses     []string
	blockedText  string
}

type document struct {
	id            string
	slug          string
	title         string
	summary       string
	content       string
	url           string
	teamID        string
	tags          []string
	owner         string
	visibility    string
	updatedInDays int
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
			id: userIgor, email: "igor@demo.local", fullName: "Игорь Белов",
			role: "mentor", teamID: teamAds, position: "Ведущий разработчик",
		},
		{
			id: userMasha, email: "masha@demo.local", fullName: "Мария Кузнецова",
			role: "intern", teamID: teamAds, mentorID: userIgor,
			position: "Стажёр-разработчик",
		},
		{
			id: userDasha, email: "dasha@demo.local", fullName: "Дарья Соколова",
			role: "intern", teamID: teamSearch, mentorID: userPetr,
			position: "Стажёр-разработчик",
		},
		{
			id: userOlga, email: "olga@demo.local", fullName: "Ольга Кузнецова",
			role: "manager", teamID: teamSearch, position: "Руководитель команды",
		},
		{
			id: userMarina, email: "marina@demo.local", fullName: "Марина Белова",
			role: "hr", position: "HR-партнёр",
		},
		{
			id: userTimur, email: "timur@demo.local", fullName: "Тимур Абрамов",
			role: "intern", teamID: teamSearch, mentorID: userPetr,
			position: "Стажёр-разработчик",
		},
		{
			id: userVera, email: "vera@demo.local", fullName: "Вера Соколова",
			role: "intern", teamID: teamAds, mentorID: userIgor,
			position: "Стажёр-разработчик",
		},
		{
			id: userSonya, email: "sonya@demo.local", fullName: "Соня Лебедева",
			role: "intern", teamID: teamPlatform, position: "Стажёр-разработчик",
		},
		{
			id: userKirill, email: "kirill@demo.local", fullName: "Кирилл Морозов",
			role: "intern", teamID: teamSearch, mentorID: userPetr,
			position: "Стажёр-разработчик",
		},
	}

	for _, u := range users {
		var mentor any
		if u.mentorID != "" {
			mentor = u.mentorID
		}

		var team any
		if u.teamID != "" {
			team = u.teamID
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
			u.id, u.email, u.fullName, u.role, team, mentor, u.position,
		)
		if err != nil {
			return fmt.Errorf("seed: upsert user %s: %w", u.email, err)
		}
	}
	return nil
}

func seedOnboarding(ctx context.Context, tx pgx.Tx) error {
	profiles := []internPlan{
		{
			userID: userDasha, planID: planDasha, fixedIDs: true,
			title: "Адаптация в команде Search", startsInDays: -3,
			statuses: []string{
				"done", "done", "in_progress", "done", "done",
				"pending", "blocked", "pending", "in_progress", "pending",
			},
			blockedText: "Ждём подтверждения от HR",
		},
		{
			userID: userMasha, title: "Адаптация в команде Ads", startsInDays: -12,
			statuses: []string{
				"done", "done", "done", "done", "in_progress",
				"pending", "pending", "pending", "pending", "pending",
			},
			blockedText: "",
		},
		{
			userID: userTimur, title: "Адаптация в команде Search", startsInDays: -6,
			statuses: []string{
				"done", "done", "done", "done", "done",
				"done", "in_progress", "pending", "pending", "pending",
			},
			blockedText: "",
		},
		{
			userID: userVera, title: "Адаптация в команде Ads", startsInDays: -9,
			statuses: []string{
				"blocked", "pending", "pending", "pending", "pending",
				"pending", "pending", "pending", "pending", "pending",
			},
			blockedText: "Не подтверждён доступ к VPN",
		},
		{
			userID: userSonya, title: "Адаптация в команде Platform", startsInDays: -1,
			statuses: []string{
				"done", "in_progress", "pending", "pending", "pending",
				"pending", "pending", "pending", "pending", "pending",
			},
			blockedText: "",
		},
	}

	for _, profile := range profiles {
		if err := seedPlan(ctx, tx, profile); err != nil {
			return err
		}
	}

	return nil
}

func seedPlan(ctx context.Context, tx pgx.Tx, profile internPlan) error {
	planID := profile.planID
	if planID == "" {
		planID = derivedID("plan", profile.userID)
	}

	startsOn := time.Now().AddDate(0, 0, profile.startsInDays)

	if _, err := tx.Exec(ctx, `
		INSERT INTO onboarding_plans (id, user_id, title, starts_on)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    title = EXCLUDED.title,
		    starts_on = EXCLUDED.starts_on`,
		planID, profile.userID, profile.title, startsOn,
	); err != nil {
		return fmt.Errorf("seed: upsert onboarding plan %s: %w", planID, err)
	}

	stageIDs := make([]string, 0, len(stageTemplates))

	for index, template := range stageTemplates {
		stageID := derivedID("stage", planID+":"+strconv.Itoa(index))

		if profile.fixedIDs {
			stageID = fixedStageIDs[index]
		}

		stageIDs = append(stageIDs, stageID)

		if _, err := tx.Exec(ctx, `
			INSERT INTO onboarding_stages (id, plan_id, title, description, position)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE
			SET title = EXCLUDED.title,
			    description = EXCLUDED.description,
			    position = EXCLUDED.position`,
			stageID, planID, template.title, template.description, index+1,
		); err != nil {
			return fmt.Errorf("seed: upsert onboarding stage %s: %w", stageID, err)
		}
	}

	for index, template := range itemTemplates {
		itemID := derivedID("item", planID+":"+strconv.Itoa(index))

		if profile.fixedIDs {
			itemID = fmt.Sprintf("44444444-4444-4444-4444-4444444444%02d", index+1)
		}

		status := "pending"
		if index < len(profile.statuses) {
			status = profile.statuses[index]
		}

		blockedReason := ""
		if status == "blocked" {
			blockedReason = profile.blockedText
		}

		var due any
		if template.dueInDays != 0 {
			due = time.Now().AddDate(0, 0, template.dueInDays)
		}

		var blocked any
		if blockedReason != "" {
			blocked = blockedReason
		}

		if _, err := tx.Exec(ctx, `
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
			itemID, planID, stageIDs[template.stageIndex], template.title, template.description,
			template.kind, status, blocked, due, template.position,
		); err != nil {
			return fmt.Errorf("seed: upsert onboarding item %s: %w", itemID, err)
		}
	}

	return nil
}

func derivedID(kind, seed string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(kind+":"+seed)).String()
}

func seedKnowledgeBase(ctx context.Context, tx pgx.Tx) error {
	documents := []document{
		{
			id: "55555555-5555-5555-5555-555555555501", slug: "jira-access",
			title:   "Как получить доступ к Jira",
			summary: "Пошаговая инструкция для стажёров команды Search.",
			content: "1. Напишите наставнику в чат.\n2. Наставник создаёт заявку в IT-портале.\n3. Доступ приходит на рабочую почту в течение дня.\n4. Войдите в Jira через SSO.",
			url:     "https://demo.local/docs/jira-access", teamID: teamSearch,
			tags:  []string{"jira", "доступ", "access", "search"},
			owner: "petr@demo.local", visibility: "team",
		},
		{
			id: "55555555-5555-5555-5555-555555555502", slug: "vpn-setup",
			title:   "Настройка VPN",
			summary: "Как подключиться к корпоративной сети с ноутбука.",
			content: "Установите клиент из IT-портала, импортируйте профиль и войдите под рабочей учётной записью. При проблемах пишите в #it-help.",
			url:     "https://demo.local/docs/vpn-setup", teamID: teamPlatform,
			tags:  []string{"vpn", "сеть", "it"},
			owner: "admin@demo.local", visibility: "team",
		},
		{
			id: "55555555-5555-5555-5555-555555555503", slug: "code-review-guide",
			title:   "Гайд по код-ревью",
			summary: "Правила ревью и ожидания от автора PR.",
			content: "Держите PR небольшим (до 400 строк), описывайте изменения и добавляйте тесты. Ревьюер отвечает в течение рабочего дня.",
			url:     "https://demo.local/docs/code-review", teamID: teamSearch,
			tags:  []string{"ревью", "pr", "процессы"},
			owner: "petr@demo.local", visibility: "team",
		},
		{
			id: "55555555-5555-5555-5555-555555555504", slug: "release-process",
			title:   "Процесс релизов",
			summary: "Ветки, стенды и деплой в прод.",
			content: "Разработка идёт в main, релиз — по расписанию два раза в неделю. Перед деплоем прогоняются тесты и проверка на стенде stage.",
			url:     "https://demo.local/docs/release-process", teamID: teamPlatform,
			tags:  []string{"релиз", "деплой", "процессы"},
			owner: "admin@demo.local", visibility: "team",
		},
		{
			id: "55555555-5555-5555-5555-555555555505", slug: "vacation-policy",
			title:   "Отпуск и отгулы",
			summary: "Сколько дней доступно и как их согласовать.",
			content: "Стажёру доступны оплачиваемые дни отпуска согласно договору. Заявка создаётся в HR-системе и согласуется с наставником за две недели.",
			url:     "https://demo.local/docs/vacation", teamID: "",
			tags:  []string{"отпуск", "hr", "отгул"},
			owner: "marina@demo.local", visibility: "public",
		},
		{
			id: "55555555-5555-5555-5555-555555555506", slug: "onboarding-checklist",
			title:   "Чек-лист новичка",
			summary: "Какие шаги ждут стажёра в первые недели.",
			content: "Первый день: ноутбук, VPN и знакомство с командой. Первая неделя: доступы, 1:1 с наставником, первая задача. Второй неделя: ревью, релиз, деплой.",
			url:     "https://demo.local/docs/onboarding", teamID: teamSearch,
			tags:  []string{"онбординг", "чек-лист", "адаптация"},
			owner: "marina@demo.local", visibility: "team",
		},
		{
			id: "55555555-5555-5555-5555-555555555507", slug: "first-pr",
			title:   "Как открыть первый PR",
			summary: "Минимальный путь от форка до мержа.",
			content: "Сделайте форк, создайте ветку от main, внесите небольшое изменение с тестом и откройте PR. Ревьюер оставит комментарии, после правок PR смержится.",
			url:     "https://demo.local/docs/first-pr", teamID: teamSearch,
			tags:  []string{"pr", "git", "первый код"},
			owner: "petr@demo.local", visibility: "team",
		},
		{
			id: "55555555-5555-5555-5555-555555555508", slug: "standup-rules",
			title:   "Как проходят дейлики",
			summary: "Формат, время и правило отсутствия.",
			content: "Дейлик каждый день в 10:00 по московскому времени, формат — три вопроса: что сделал, что делаешь, что мешает. Отсутствовать без предупреждения нельзя.",
			url:     "https://demo.local/docs/standup", teamID: "",
			tags:  []string{"дейлик", "процессы", "команда"},
			owner: "marina@demo.local", visibility: "public",
		},
		{
			id: "55555555-5555-5555-5555-555555555509", slug: "secrets-policy",
			title:   "Правила работы с секретами",
			summary: "Где хранить пароли и токены.",
			content: "Пароли и токены храним только в менеджере секретов. Запрещено класть их в репозиторий, конфиги и переписку. Обнаружили утечку — сразу сообщите в #security.",
			url:     "https://demo.local/docs/secrets", teamID: "",
			tags:  []string{"секреты", "security", "доступ"},
			owner: "admin@demo.local", visibility: "hr", updatedInDays: -200,
		},
		{
			id: "55555555-5555-5555-5555-555555555510", slug: "benefits-2026",
			title:   "Льготы и бенефиты",
			summary: "Что доступно сотрудникам и как оформить.",
			content: "Сотрудникам доступны ДМС со стоматологией, обучение и компенсация спортзала. Оформление — через HR-портал, срок рассмотрения — пять рабочих дней.",
			url:     "https://demo.local/docs/benefits", teamID: "",
			tags:  []string{"бенефиты", "hr", "дмс"},
			owner: "marina@demo.local", visibility: "hr",
		},
		{
			id: "55555555-5555-5555-5555-555555555511", slug: "deploy-runbook",
			title:   "Runbook деплоя",
			summary: "Порядок выката и откат.",
			content: "Выкат идёт по расписанию: стенд, smoke-тесты, затем прод с двойным подтверждением. Откат — одной командой, после чего инцидент разбирается в течение дня.",
			url:     "https://demo.local/docs/deploy-runbook", teamID: teamAds,
			tags:  []string{"деплой", "runbook", "stage"},
			owner: "igor@demo.local", visibility: "team",
		},
		{
			id: "55555555-5555-5555-5555-555555555512", slug: "access-bundle",
			title:   "Пакет доступов стажёра",
			summary: "Какие доступы получает стажёр на старте.",
			content: "Базовый пакет стажёра: VPN, Jira, репозиторий команды и база знаний. Дополнительно: доступ к стендам — после первого merged PR.",
			url:     "https://demo.local/docs/access-bundle", teamID: teamSearch,
			tags:  []string{"доступ", "jira", "vpn", "пакет"},
			owner: "petr@demo.local", visibility: "team",
		},
	}

	for _, d := range documents {
		var teamID any
		if d.teamID != "" {
			teamID = d.teamID
		}

		visibility := d.visibility
		if visibility == "" {
			visibility = "team"
		}

		updatedAt := time.Now()
		if d.updatedInDays != 0 {
			updatedAt = time.Now().AddDate(0, 0, d.updatedInDays)
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO kb_documents (id, slug, title, summary, content, url, team_id, tags, owner_email, visibility, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (id) DO UPDATE
			SET slug = EXCLUDED.slug,
			    title = EXCLUDED.title,
			    summary = EXCLUDED.summary,
			    content = EXCLUDED.content,
			    url = EXCLUDED.url,
			    team_id = EXCLUDED.team_id,
			    tags = EXCLUDED.tags,
			    owner_email = EXCLUDED.owner_email,
			    visibility = EXCLUDED.visibility,
			    updated_at = EXCLUDED.updated_at`,
			d.id, d.slug, d.title, d.summary, d.content, d.url, teamID, d.tags,
			d.owner, visibility, updatedAt,
		)
		if err != nil {
			return fmt.Errorf("seed: upsert kb document %s: %w", d.slug, err)
		}

		if err := seedDocumentChunks(ctx, tx, d.id, d.content); err != nil {
			return err
		}
	}

	return nil
}

func seedDocumentChunks(ctx context.Context, tx pgx.Tx, documentID, content string) error {
	parts := strings.Split(content, "\n")

	for index, part := range parts {
		chunk := strings.TrimSpace(part)
		if chunk == "" {
			continue
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO doc_chunks (document_id, chunk_index, content)
			VALUES ($1, $2, $3)
			ON CONFLICT (document_id, chunk_index) DO UPDATE
			SET content = EXCLUDED.content`,
			documentID, index, chunk,
		)
		if err != nil {
			return fmt.Errorf("seed: upsert doc chunk %s/%d: %w", documentID, index, err)
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
