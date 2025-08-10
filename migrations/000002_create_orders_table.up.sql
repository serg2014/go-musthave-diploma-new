DO $$ BEGIN
    CREATE TYPE order_status AS ENUM ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS orders (
    order_id text NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL,
    upload_time timestamp NOT NULL DEFAULT current_timestamp,
    status  order_status NOT NULL,
    accrual int
);
CREATE INDEX orders_user_id_idx ON orders (user_id);

CREATE TABLE IF NOT EXISTS orders_for_process (
    order_id text NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL,
    who_lock char(20),
    locked_at timestamp,
    update_time timestamp NOT NULL
);
CREATE INDEX orders_for_process_who_lock_update_time_idx ON orders_for_process (who_lock, update_time);
CREATE INDEX orders_for_process_locked_at_idx ON orders_for_process (locked_at);
