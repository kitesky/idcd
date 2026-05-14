CREATE TABLE oncall_schedules (
  id            TEXT PRIMARY KEY,
  team_id       TEXT NOT NULL,
  name          TEXT NOT NULL,
  rotation_type TEXT NOT NULL DEFAULT 'weekly',
  handoff_hour  INTEGER NOT NULL DEFAULT 9,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON oncall_schedules(team_id);

CREATE TABLE oncall_participants (
  id            TEXT PRIMARY KEY,
  schedule_id   TEXT NOT NULL,
  user_id       TEXT NOT NULL,
  order_index   INTEGER NOT NULL,
  UNIQUE(schedule_id, order_index)
);
CREATE INDEX ON oncall_participants(schedule_id);

CREATE TABLE oncall_overrides (
  id            TEXT PRIMARY KEY,
  schedule_id   TEXT NOT NULL,
  user_id       TEXT NOT NULL,
  start_at      TIMESTAMPTZ NOT NULL,
  end_at        TIMESTAMPTZ NOT NULL,
  created_by    TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON oncall_overrides(schedule_id, start_at, end_at);
