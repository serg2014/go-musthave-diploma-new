DO $$ BEGIN
    CREATE TYPE debet_credit_type AS ENUM ('DEBET', 'CREDIT');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS debet_credit (
    order_id text NOT NULL,
    type debet_credit_type ,
    user_id uuid NOT NULL,
    sum int NOT NULL,
    create_time timestamp NOT NULL DEFAULT current_timestamp,
    PRIMARY KEY(order_id, type)
);
CREATE INDEX debet_credit_user_id_type_idx ON debet_credit (user_id, type);