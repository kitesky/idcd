-- +goose Up

-- 00039_i18n_locale_short_codes.sql — i18n Phase 2c (DB migrations) ①
--
-- 把 users.locale 从历史 BCP 47 值（'zh-CN', 'en-US' 等）转换为
-- shared registry 的内部短码（'cn' / 'en'）。详 docs/prd/I18N-PLAN.md §2.6。
--
-- 决策记录：
--   1. 不加 CHECK constraint。
--      原因：未来加新 locale (e.g. 'ja') 只需在 config/locales.json + messages/
--      目录里追加，零代码改动；如果 DB 也写 CHECK，加新语言会触发一次
--      ALTER TABLE DROP CONSTRAINT / ADD CHECK，违背 i18n 计划"零代码改动加
--      新语言"的核心原则。应用层在 repository.Create/Update 时调
--      i18n.MustDefault().IsSupported() 校验，足够。
--   2. 兜底分支把未识别值（NULL / '' / 已被弃用的 'fr-FR' 等历史脏数据）一律
--      映射到 'cn'（系统默认 locale），不丢用户体验。
--
-- 顺序：先映射已知组，最后兜底，避免误覆盖。

UPDATE "user"
   SET locale = 'cn'
 WHERE locale IN ('zh-CN', 'zh', 'zh-Hans', 'zh-Hans-CN', 'zh-SG', 'zh-TW', 'zh-Hant');

UPDATE "user"
   SET locale = 'en'
 WHERE locale IN ('en-US', 'en-GB', 'en-AU', 'en-CA', 'en');

-- 任何不在 ('cn', 'en') 的剩余值统一兜底为 'cn'。
-- 当未来引入第三种短码（如 'ja'）时，新增对应 UPDATE 行即可，无需改本迁移。
UPDATE "user"
   SET locale = 'cn'
 WHERE locale NOT IN ('cn', 'en');

-- 调整 DEFAULT 为短码，保证后续 INSERT 自动得到合法值。
ALTER TABLE "user" ALTER COLUMN locale SET DEFAULT 'cn';

-- +goose Down

-- 回滚：恢复历史 DEFAULT。不撤销数据 UPDATE —— 历史 BCP 47 值无法从短码
-- 反推（'cn' 既可能源自 'zh-CN' 也可能源自 'zh-Hans'），强行回滚反而引入新错
-- 误。如需精确回滚，请提前 SELECT 全表 COPY OUT 备份。
ALTER TABLE "user" ALTER COLUMN locale SET DEFAULT 'zh-CN';
