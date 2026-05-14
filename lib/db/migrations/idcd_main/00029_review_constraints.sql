-- +goose Up
-- Partial UNIQUE index: prevents concurrent aggregators creating duplicate firing alerts.
CREATE UNIQUE INDEX IF NOT EXISTS uq_alert_events_firing
  ON alert_events(monitor_id, policy_id) WHERE status = 'firing';

-- Referral deduplication: one reward row per (referrer, referred) pair.
ALTER TABLE referral_rewards
  ADD CONSTRAINT uq_referral_rewards_pair UNIQUE (referrer_id, referred_id);

-- Status page subscription deduplication: prevents duplicate notifications.
ALTER TABLE status_page_subscriptions
  ADD CONSTRAINT uq_status_subscriptions_endpoint
  UNIQUE (status_page_id, channel_type, endpoint);

-- Anchor deviations: one open deviation per monitor+type pair.
CREATE UNIQUE INDEX IF NOT EXISTS uq_anchor_deviations_open
  ON anchor_deviations(monitor_id, deviation_type) WHERE status = 'open';

-- CHECK constraints: text enum columns without constraints.
ALTER TABLE api_key
  ADD CONSTRAINT chk_api_key_key_type CHECK (key_type IN ('production', 'test'));

ALTER TABLE beta_invitations
  ADD CONSTRAINT chk_beta_invitations_status
  CHECK (status IN ('pending', 'approved', 'rejected', 'used', 'expired'));

ALTER TABLE referral_rewards
  ADD CONSTRAINT chk_referral_rewards_status
  CHECK (status IN ('pending', 'credited', 'cancelled'));

ALTER TABLE teams
  ADD CONSTRAINT chk_teams_plan CHECK (plan IN ('free', 'pro', 'enterprise'));

ALTER TABLE team_memberships
  ADD CONSTRAINT chk_team_memberships_role CHECK (role IN ('owner', 'admin', 'member'));

ALTER TABLE team_invitations
  ADD CONSTRAINT chk_team_invitations_role CHECK (role IN ('owner', 'admin', 'member'));

ALTER TABLE team_invitations
  ADD CONSTRAINT chk_team_invitations_status
  CHECK (status IN ('pending', 'accepted', 'revoked', 'expired'));

ALTER TABLE oncall_schedules
  ADD CONSTRAINT chk_oncall_schedules_rotation_type
  CHECK (rotation_type IN ('weekly', 'daily', 'custom'));

ALTER TABLE incident_postmortems
  ADD CONSTRAINT chk_postmortems_status
  CHECK (status IN ('draft', 'in_review', 'published'));

ALTER TABLE incident_postmortems
  ADD CONSTRAINT chk_postmortems_severity
  CHECK (severity IN ('low', 'medium', 'high', 'critical'));

ALTER TABLE node_applications
  ADD CONSTRAINT chk_node_applications_status
  CHECK (status IN ('pending', 'approved', 'rejected', 'probation', 'active', 'suspended'));

ALTER TABLE anchor_deviations
  ADD CONSTRAINT chk_anchor_deviations_deviation_type
  CHECK (deviation_type IN ('latency', 'error_rate', 'availability'));

ALTER TABLE anchor_deviations
  ADD CONSTRAINT chk_anchor_deviations_severity
  CHECK (severity IN ('low', 'medium', 'high', 'critical'));

ALTER TABLE status_page_subscriptions
  ADD CONSTRAINT chk_status_subscriptions_channel_type
  CHECK (channel_type IN ('email', 'webhook', 'wecom', 'dingtalk', 'slack'));

ALTER TABLE monitor_agent_obs_checks
  ADD CONSTRAINT chk_monitor_agent_obs_checks_status
  CHECK (status IN ('ok', 'error', 'timeout', 'degraded'));

ALTER TABLE oncall_overrides
  ADD CONSTRAINT chk_oncall_overrides_times CHECK (end_at > start_at);

-- Missing performance indexes.
CREATE INDEX IF NOT EXISTS idx_oncall_participants_user_id ON oncall_participants(user_id);
CREATE INDEX IF NOT EXISTS idx_oncall_overrides_active ON oncall_overrides(schedule_id, start_at, end_at) WHERE end_at > NOW();
CREATE INDEX IF NOT EXISTS idx_point_redemptions_user_id ON point_redemptions(user_id);
CREATE INDEX IF NOT EXISTS idx_anchor_deviations_baseline_id ON anchor_deviations(baseline_id);
CREATE INDEX IF NOT EXISTS idx_webauthn_challenges_user_expires ON webauthn_challenges(user_id, expires_at);
CREATE INDEX IF NOT EXISTS idx_team_invitations_email ON team_invitations(email);
CREATE UNIQUE INDEX IF NOT EXISTS uq_team_invitations_pending ON team_invitations(team_id, email) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_incident_postmortems_monitor_id ON incident_postmortems(monitor_id);
CREATE INDEX IF NOT EXISTS idx_alert_groups_user_id ON alert_groups(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_alert_groups_user_dim ON alert_groups(user_id, group_by, group_value);
CREATE INDEX IF NOT EXISTS idx_pat_active ON personal_access_tokens(user_id) WHERE expires_at IS NULL OR expires_at > NOW();

-- +goose Down
DROP INDEX IF EXISTS uq_alert_events_firing;
ALTER TABLE referral_rewards DROP CONSTRAINT IF EXISTS uq_referral_rewards_pair;
ALTER TABLE status_page_subscriptions DROP CONSTRAINT IF EXISTS uq_status_subscriptions_endpoint;
DROP INDEX IF EXISTS uq_anchor_deviations_open;
ALTER TABLE api_key DROP CONSTRAINT IF EXISTS chk_api_key_key_type;
ALTER TABLE beta_invitations DROP CONSTRAINT IF EXISTS chk_beta_invitations_status;
ALTER TABLE referral_rewards DROP CONSTRAINT IF EXISTS chk_referral_rewards_status;
ALTER TABLE teams DROP CONSTRAINT IF EXISTS chk_teams_plan;
ALTER TABLE team_memberships DROP CONSTRAINT IF EXISTS chk_team_memberships_role;
ALTER TABLE team_invitations DROP CONSTRAINT IF EXISTS chk_team_invitations_role;
ALTER TABLE team_invitations DROP CONSTRAINT IF EXISTS chk_team_invitations_status;
ALTER TABLE oncall_schedules DROP CONSTRAINT IF EXISTS chk_oncall_schedules_rotation_type;
ALTER TABLE incident_postmortems DROP CONSTRAINT IF EXISTS chk_postmortems_status;
ALTER TABLE incident_postmortems DROP CONSTRAINT IF EXISTS chk_postmortems_severity;
ALTER TABLE node_applications DROP CONSTRAINT IF EXISTS chk_node_applications_status;
ALTER TABLE anchor_deviations DROP CONSTRAINT IF EXISTS chk_anchor_deviations_deviation_type;
ALTER TABLE anchor_deviations DROP CONSTRAINT IF EXISTS chk_anchor_deviations_severity;
ALTER TABLE status_page_subscriptions DROP CONSTRAINT IF EXISTS chk_status_subscriptions_channel_type;
ALTER TABLE monitor_agent_obs_checks DROP CONSTRAINT IF EXISTS chk_monitor_agent_obs_checks_status;
ALTER TABLE oncall_overrides DROP CONSTRAINT IF EXISTS chk_oncall_overrides_times;
DROP INDEX IF EXISTS idx_oncall_participants_user_id;
DROP INDEX IF EXISTS idx_oncall_overrides_active;
DROP INDEX IF EXISTS idx_point_redemptions_user_id;
DROP INDEX IF EXISTS idx_anchor_deviations_baseline_id;
DROP INDEX IF EXISTS idx_webauthn_challenges_user_expires;
DROP INDEX IF EXISTS idx_team_invitations_email;
DROP INDEX IF EXISTS uq_team_invitations_pending;
DROP INDEX IF EXISTS idx_incident_postmortems_monitor_id;
DROP INDEX IF EXISTS idx_alert_groups_user_id;
DROP INDEX IF EXISTS uq_alert_groups_user_dim;
DROP INDEX IF EXISTS idx_pat_active;
