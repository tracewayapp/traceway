ALTER TABLE check_incidents ADD COLUMN status_page_id INT REFERENCES status_pages(id) ON DELETE CASCADE
