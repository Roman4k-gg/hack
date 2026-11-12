package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmailRequired = errors.New("auth: email is required")
	ErrInvalidEmail  = errors.New("auth: email is invalid")
	ErrUserNotFound  = errors.New("auth: user not found")
)

type User struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	FullName   string `json:"full_name"`
	Role       string `json:"role"`
	Position   string `json:"position"`
	TeamID     string `json:"team_id,omitempty"`
	TeamName   string `json:"team_name,omitempty"`
	TeamSlug   string `json:"team_slug,omitempty"`
	MentorID   string `json:"mentor_id,omitempty"`
	MentorName string `json:"mentor_name,omitempty"`
}

type UserStore interface {
	UserByEmail(ctx context.Context, email string) (User, error)
	UserByID(ctx context.Context, id string) (User, error)
}

type PostgresUserStore struct {
	pool *pgxpool.Pool
}

func NewPostgresUserStore(pool *pgxpool.Pool) *PostgresUserStore {
	return &PostgresUserStore{pool: pool}
}

const userColumns = `
	u.id::text,
	u.email,
	u.full_name,
	u.role,
	u.position,
	COALESCE(t.id::text, ''),
	COALESCE(t.name, ''),
	COALESCE(t.slug, ''),
	COALESCE(m.id::text, ''),
	COALESCE(m.full_name, '')`

func (s *PostgresUserStore) UserByEmail(ctx context.Context, email string) (User, error) {
	return s.fetch(ctx, "WHERE lower(u.email) = lower($1)", email)
}

func (s *PostgresUserStore) UserByID(ctx context.Context, id string) (User, error) {
	return s.fetch(ctx, "WHERE u.id::text = $1", id)
}

func (s *PostgresUserStore) fetch(ctx context.Context, where string, arg string) (User, error) {
	if s == nil || s.pool == nil {
		return User{}, fmt.Errorf("auth: postgres pool is not initialised")
	}

	query := `SELECT ` + userColumns + `
		FROM users u
		LEFT JOIN teams t ON t.id = u.team_id
		LEFT JOIN users m ON m.id = u.mentor_id
		` + where

	var user User
	err := s.pool.QueryRow(ctx, query, arg).Scan(
		&user.ID,
		&user.Email,
		&user.FullName,
		&user.Role,
		&user.Position,
		&user.TeamID,
		&user.TeamName,
		&user.TeamSlug,
		&user.MentorID,
		&user.MentorName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("auth: load user: %w", err)
	}

	return user, nil
}

type Session struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}

type Service struct {
	store  UserStore
	tokens *TokenManager
}

func NewService(store UserStore, tokens *TokenManager) *Service {
	return &Service{store: store, tokens: tokens}
}

func (s *Service) Login(ctx context.Context, email string) (Session, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return Session{}, ErrEmailRequired
	}
	if !strings.Contains(email, "@") {
		return Session{}, ErrInvalidEmail
	}

	user, err := s.store.UserByEmail(ctx, email)
	if err != nil {
		return Session{}, err
	}

	token, expiresAt, err := s.tokens.Issue(user.ID, user.Email, user.Role, user.TeamID)
	if err != nil {
		return Session{}, err
	}

	return Session{
		Token:     token,
		TokenType: "Bearer",
		ExpiresAt: expiresAt,
		User:      user,
	}, nil
}

func (s *Service) Me(ctx context.Context, userID string) (User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return User{}, ErrUserNotFound
	}
	return s.store.UserByID(ctx, userID)
}
