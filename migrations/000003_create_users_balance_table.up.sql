CREATE TABLE IF NOT EXISTS users_balance (
    user_id uuid PRIMARY KEY,
    accrual int NOT NULL DEFAULT 0,
    withdrawn int NOT NULL DEFAULT 0
);