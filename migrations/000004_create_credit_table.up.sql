CREATE TABLE IF NOT EXISTS credit (
    order_id text PRIMARY KEY,
    user_id uuid NOT NULL,
    sum int NOT NULL,
    create_time timestamp NOT NULL
);
-- TODO возможно достаточно одного индекса user_id, order_id
CREATE INDEX credit_user_id_idx ON credit (user_id);