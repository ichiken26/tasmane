BEGIN;
-- メモリンク: メモアプリのメモID（memo-yyyymmdd-名前）に含まれる日付を保持し、表示タイトルは名前だけにする

ALTER TABLE task_memo_links ADD COLUMN memo_date date;

-- 既存行の補完（URL末尾のメモIDから日付、IDそのままのタイトルは名前だけへ）
UPDATE task_memo_links
SET memo_date = to_date(substring(url FROM '/memo-([0-9]{8})-[^/?#]+(?:[?#].*)?$'), 'YYYYMMDD')
WHERE memo_date IS NULL AND url ~ '/memo-[0-9]{8}-[^/?#]+(?:[?#].*)?$';

UPDATE task_memo_links
SET title = substring(title FROM '^memo-[0-9]{8}-(.+)$')
WHERE title ~ '^memo-[0-9]{8}-.+$';

COMMIT;
