package feed

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Card is one feed card as stored.
type Card struct {
	ID          int64
	Kind        string
	Title       string
	Body        string
	ImageData   string // data:image/jpeg;base64,...
	PublishedAt time.Time
	Author      Author
}

// Author is a card's author.
type Author struct {
	ID          int64
	DisplayName string
	AvatarData  string // data:image/png;base64,...
}

// Profile is an author's public profile.
type Profile struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"displayName"`
	Bio         string `json:"bio"`
	Followers   int64  `json:"followers"`
}

// Store reads cards and authors.
type Store interface {
	Latest(ctx context.Context, audiences []string, limit int) ([]Card, error)
	Card(ctx context.Context, id int64) (*Card, error)
	Profile(ctx context.Context, id int64) (*Profile, error)
}

// SQL implements Store on PostgreSQL.
type SQL struct{ DB *sql.DB }

// Latest returns the newest cards for the given audiences.
func (s *SQL) Latest(ctx context.Context, audiences []string, limit int) ([]Card, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT c.id, c.kind, c.title, c.body, c.image_data, c.published_at, a.id, a.display_name, a.avatar_data FROM feed_cards c JOIN authors a ON a.id = c.author_id WHERE c.audience = ANY($1) ORDER BY c.published_at DESC LIMIT $2`, audiences, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Card
	for rows.Next() {
		var c Card
		var image, avatar sql.NullString
		if err := rows.Scan(&c.ID, &c.Kind, &c.Title, &c.Body, &image, &c.PublishedAt, &c.Author.ID, &c.Author.DisplayName, &avatar); err != nil {
			return nil, err
		}
		c.ImageData, c.Author.AvatarData = image.String, avatar.String
		out = append(out, c)
	}
	return out, rows.Err()
}

// Card returns one card with its author's name, or nil.
func (s *SQL) Card(ctx context.Context, id int64) (*Card, error) {
	var c Card
	var image sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT c.id, c.kind, c.title, c.body, c.image_data, c.published_at, a.id, a.display_name FROM feed_cards c JOIN authors a ON a.id = c.author_id WHERE c.id = $1`, id).
		Scan(&c.ID, &c.Kind, &c.Title, &c.Body, &image, &c.PublishedAt, &c.Author.ID, &c.Author.DisplayName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.ImageData = image.String
	return &c, nil
}

// Profile returns an author's profile, or nil.
func (s *SQL) Profile(ctx context.Context, id int64) (*Profile, error) {
	var p Profile
	err := s.DB.QueryRowContext(ctx, `SELECT id, display_name, bio, followers FROM authors WHERE id = $1`, id).
		Scan(&p.ID, &p.DisplayName, &p.Bio, &p.Followers)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}
