INSERT INTO projects (id, name, token, framework, created_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'Default Project', 'default_token_change_me', 'custom', NOW())
ON CONFLICT (id) DO NOTHING
