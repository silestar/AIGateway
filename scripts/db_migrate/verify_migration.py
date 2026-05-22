#!/usr/bin/env python3
"""
迁移数据校验工具 — 通过 docker exec psql 查询 PG 进行比对

用法:
  1. 编辑下面的 SOURCE_DB_PATH 和 PG_CONFIG
  2. python3 verify_migration.py
    
依赖:
  - 目标容器需安装 postgresql-client (apk add postgresql-client)
  - 脚本通过 docker exec <agw-dev> psql 查询 PG
"""

import sqlite3
import subprocess

# ─── 配置 ─────────────────────────────────────────────────────
SOURCE_DB_PATH = "/tmp/agw_source_test.db"

# 目标 PG 连接信息（与 db_migrate.py 的 target 配置保持一致）
PG_CONTAINER = "agw-dev"
PG_HOST = "PostgreSql"
PG_USER = "agw_dev"
PG_PASS = "5RpC5CNHFZKjWFD5"
PG_DB = "agw_dev"
# ──────────────────────────────────────────────────────────────


def pg_count(table):
    cmd = (
        f"PGPASSWORD={PG_PASS} psql -h {PG_HOST} -U {PG_USER} -d {PG_DB} "
        f'-tAc \'SELECT COUNT(*) FROM "{table}"\' 2>/dev/null'
    )
    result = subprocess.run(
        ["docker", "exec", PG_CONTAINER, "sh", "-c", cmd],
        capture_output=True, text=True, timeout=15,
    )
    if result.returncode != 0:
        return None, "exec failed"
    val = result.stdout.strip()
    try:
        return int(val), None
    except (ValueError, TypeError):
        return None, f"bad output: {val[:50]}"


def main():
    src = sqlite3.connect(f"file:{SOURCE_DB_PATH}?mode=ro", uri=True)
    src.row_factory = sqlite3.Row

    src_tables = sorted(
        r[0]
        for r in src.execute(
            "SELECT name FROM sqlite_master "
            "WHERE type='table' AND name NOT LIKE 'sqlite_%'"
        ).fetchall()
    )

    print("=" * 60)
    print("  迁移数据校验  (SQLite → PostgreSQL)")
    print("=" * 60)
    print()
    print(f"  源: SQLite ({SOURCE_DB_PATH})")
    print(f"  目标: PostgreSQL ({PG_HOST}:5432/{PG_DB})")
    print()

    total_src = 0
    total_tgt = 0
    ok_count = 0
    bad_tables = []

    for table in src_tables:
        src_count = src.execute(f'SELECT COUNT(*) FROM "{table}"').fetchone()[0]
        tgt_count, err = pg_count(table)

        if isinstance(src_count, int) and isinstance(tgt_count, int):
            total_src += src_count
            total_tgt += tgt_count

        status = "✅" if src_count == tgt_count else "❌"
        if status == "❌":
            bad_tables.append((table, src_count, tgt_count, err))
        else:
            ok_count += 1

        tgt_display = (
            str(tgt_count)
            if isinstance(tgt_count, int)
            else f"ERR({err})"
        )
        print(
            f"  {status} {table:30s}  "
            f"SQLite: {src_count:>6}  |  PG: {tgt_display:>10}"
        )

    print()
    print("-" * 60)
    print(
        f"  源总行数: {total_src}  |  目标总行数: {total_tgt}  "
        f"|  匹配: {ok_count}/{len(src_tables)}"
    )

    if bad_tables:
        print(f"  ❌ 不匹配的表 ({len(bad_tables)}):")
        for t_name, s, tgt, err_msg in bad_tables:
            extra = f" ({err_msg})" if err_msg else ""
            print(f"     - {t_name}: SQLite={s}, PG={tgt}{extra}")
    else:
        print(f"  🎉 全部 {len(src_tables)} 张表行数一致！")

    print("=" * 60)
    src.close()


if __name__ == "__main__":
    main()