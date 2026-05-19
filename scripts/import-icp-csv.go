//go:build ignore

// import-icp-csv 把 CSV 批量导入 icp.records。
//
// CSV 格式(无需 header,7 列,UTF-8):
//
//	domain,icp_number,company,filing_type,filed_at(YYYY-MM-DD),source,note
//
// 例:
//
//	baidu.com,京ICP证030173号-1,北京百度网讯科技有限公司,企业,2003-04-22,seed,
//	taobao.com,浙B2-20080224-1,浙江淘宝网络有限公司,企业,2008-03-12,seed,
//
// 用法(读 config/dev.env.yaml 取 DSN):
//
//	cd /Volumes/Workspace/code/idcd
//	DEV_DB_DSN=$(python3 -c "import yaml;c=yaml.safe_load(open('config/dev.env.yaml'));print(c['database']['main']['dsn'])") \
//	  go run scripts/import-icp-csv.go scripts/seed-icp.csv
//
// 同一 domain 会 upsert(以 CSV 行为准),不去手动改库;source 字段用来区分
// 数据来源(seed / miit_export / manual / scraper_xxx),便于后续按来源清洗。

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: go run scripts/import-icp-csv.go <csv-file>")
	}
	dsn := os.Getenv("DEV_DB_DSN")
	if dsn == "" {
		log.Fatal("DEV_DB_DSN env var required (DSN to idcd_main)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	q := idcdmain.New(pool)

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("open csv: %v", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // 容忍 note 列缺省

	var ok, skipped int
	for line := 1; ; line++ {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("line %d: %v", line, err)
		}
		if len(row) < 6 {
			log.Printf("line %d: skip (need ≥6 cols, got %d)", line, len(row))
			skipped++
			continue
		}
		// 跳过 header / 空行
		if line == 1 && strings.EqualFold(strings.TrimSpace(row[0]), "domain") {
			continue
		}
		if strings.TrimSpace(row[0]) == "" {
			continue
		}

		params := idcdmain.UpsertICPRecordParams{
			Domain:     strings.ToLower(strings.TrimSpace(row[0])),
			IcpNumber:  strings.TrimSpace(row[1]),
			Company:    strings.TrimSpace(row[2]),
			FilingType: strings.TrimSpace(row[3]),
			Source:     strings.TrimSpace(row[5]),
		}
		if d := strings.TrimSpace(row[4]); d != "" {
			t, perr := time.Parse("2006-01-02", d)
			if perr != nil {
				log.Printf("line %d: bad date %q: %v", line, d, perr)
				skipped++
				continue
			}
			params.FiledAt = pgtype.Date{Time: t, Valid: true}
		}
		if len(row) >= 7 {
			params.Note = strings.TrimSpace(row[6])
		}
		if params.Source == "" {
			params.Source = "manual"
		}

		if _, err := q.UpsertICPRecord(ctx, params); err != nil {
			log.Printf("line %d (%s): %v", line, params.Domain, err)
			skipped++
			continue
		}
		ok++
	}
	fmt.Printf("imported %d records, skipped %d\n", ok, skipped)
}
