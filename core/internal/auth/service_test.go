package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeUserStore struct {
	users map[string]User
}

func newFakeUserStore(users ...User) *fakeUserStore {
	store := &fakeUserStore{users: map[string]User{}}
	for _, user := range users {
		store.users[user.ID] = user
	}
	return store
}

func (s *fakeUserStore) UserByEmail(_ context.Context, email string) (User, error) {
	for _, user := range s.users {
		if strings.EqualFold(user.Email, email) {
			return user, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (s *fakeUserStore) UserByID(_ context.Context, id string) (User, error) {
	user, ok := s.users[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func demoUser() User {
	return User{
		ID:         "22222222-2222-2222-2222-222222222224",
		Email:      "dasha@demo.local",
		FullName:   "Дарья Соколова",
		Role:       "intern",
		Position:   "Стажёр-разработчик",
		TeamID:     "11111111-1111-1111-1111-111111111111",
		TeamName:   "Search",
		TeamSlug:   "search",
		MentorID:   "22222222-2222-2222-2222-222222222221",
		MentorName: "Пётр Иванов",
	}
}

func TestServiceLoginIssuesToken(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager("secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)

	service := NewService(newFakeUserStore(demoUser()), manager)

	session, err := service.Login(context.Background(), " Dasha@Demo.local ")
	require.NoError(t, err)
	require.NotEmpty(t, session.Token)
	require.Equal(t, "Bearer", session.TokenType)
	require.Equal(t, "dasha@demo.local", session.User.Email)
	require.Equal(t, "intern", session.User.Role)
	require.Equal(t, "Search", session.User.TeamName)
	require.WithinDuration(t, time.Now().Add(time.Hour), session.ExpiresAt, time.Minute)

	claims, err := manager.Parse(session.Token)
	require.NoError(t, err)
	require.Equal(t, session.User.ID, claims.Subject)
}

func TestServiceLoginValidatesEmail(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager("secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)

	service := NewService(newFakeUserStore(demoUser()), manager)

	_, err = service.Login(context.Background(), "   ")
	require.ErrorIs(t, err, ErrEmailRequired)

	_, err = service.Login(context.Background(), "not-an-email")
	require.ErrorIs(t, err, ErrInvalidEmail)

	_, err = service.Login(context.Background(), "unknown@demo.local")
	require.ErrorIs(t, err, ErrUserNotFound)
}

func TestServiceMeReturnsProfile(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager("secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)

	service := NewService(newFakeUserStore(demoUser()), manager)

	user, err := service.Me(context.Background(), demoUser().ID, []string{"profile:self"})
	require.NoError(t, err)
	require.Equal(t, "Дарья Соколова", user.FullName)
	require.Equal(t, "Пётр Иванов", user.MentorName)
	require.Equal(t, []string{"profile:self"}, user.Scopes)

	_, err = service.Me(context.Background(), "unknown-id", nil)
	require.ErrorIs(t, err, ErrUserNotFound)

	_, err = service.Me(context.Background(), "  ", nil)
	require.ErrorIs(t, err, ErrUserNotFound)
}
