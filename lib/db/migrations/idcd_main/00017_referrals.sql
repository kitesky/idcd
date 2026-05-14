CREATE TABLE referral_codes (
  id         TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL UNIQUE,
  code       TEXT NOT NULL UNIQUE,
  uses_count INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON referral_codes(code);

CREATE TABLE referral_rewards (
  id            TEXT PRIMARY KEY,
  referrer_id   TEXT NOT NULL,
  referred_id   TEXT NOT NULL,
  code          TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'pending',
  reward_amount NUMERIC(10,2) NOT NULL DEFAULT 0.00,
  credited_at   TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON referral_rewards(referrer_id);
CREATE INDEX ON referral_rewards(referred_id);
