package feed

import (
	"context"
	"database/sql"
	"time"
)

// Card is one feed card as stored.
type Card struct {
	ID          int64
	Kind        string
	Title       string
	Body        string
	ImageData   string // data:image/jpeg;base64,... (about 45KB per card)
	PublishedAt time.Time
	Author      Author
}

// Author is a card's author.
type Author struct {
	ID          int64
	DisplayName string
	AvatarData  string // data:image/png;base64,...
}

// Store reads cards.
type Store interface {
	Latest(ctx context.Context, audiences []string, limit int) ([]Card, error)
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
