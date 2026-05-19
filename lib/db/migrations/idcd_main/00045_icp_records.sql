-- +goose Up

-- ICP 备案信息自建库。
-- 工信部 ICP 查询要资质 + 反爬严重,公开第三方 API(icplishi/vvhan 等)已死或
-- 鉴权。我们走"自建库 + 离线批量导入热门域名"的路:
--   * 用户查询命中 → 返回完整记录
--   * 未命中 → 前端显示「请到 beian.miit.gov.cn 手查」+ 跳转按钮
-- 后续可加管理后台 CRUD / 定时刷新 / 工信部官方授权数据。

CREATE SCHEMA IF NOT EXISTS icp;

CREATE TABLE icp.records (
  id          BIGSERIAL    PRIMARY KEY,
  -- 主域名(eTLD+1),用户查 www.baidu.com 时归一到 baidu.com 后查。
  domain      TEXT         NOT NULL UNIQUE,
  -- 备案号,如 京ICP备1234567号-1
  icp_number  TEXT         NOT NULL,
  -- 主办单位 / 公司名
  company     TEXT         NOT NULL DEFAULT '',
  -- 备案类型: 企业 / 个人 / 事业单位 / 政府机关 / ...
  filing_type TEXT         NOT NULL DEFAULT '',
  -- 审核通过日期(工信部公示)
  filed_at    DATE,
  -- 数据来源: manual / miit_export / scraper / api_xxx
  source      TEXT         NOT NULL DEFAULT 'manual',
  -- 备注(可选)
  note        TEXT         NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_icp_records_company ON icp.records (company) WHERE company <> '';

-- +goose Down
DROP TABLE IF EXISTS icp.records;
DROP SCHEMA IF EXISTS icp;
