ALTER TABLE post_mortems ADD COLUMN updated_by INTEGER REFERENCES users(id);
