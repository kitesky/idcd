-- 00011_status_page_custom_domain.sql
-- Add custom domain verification and certificate expiry tracking to status_pages.
-- D1 rule: NO cross-schema FOREIGN KEY REFERENCES.

ALTER TABLE status_pages
  ADD COLUMN IF NOT EXISTS custom_domain_verified_at   TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS custom_domain_cert_expires_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE UNIQUE INDEX IF NOT EXISTS idx_status_pages_custom_domain
  ON status_pages(custom_domain) WHERE custom_domain IS NOT NULL;
