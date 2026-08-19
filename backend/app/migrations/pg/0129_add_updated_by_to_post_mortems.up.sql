ALTER TABLE post_mortems ADD COLUMN updated_by INT REFERENCES users(id)
