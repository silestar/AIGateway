#!/usr/bin/env python3
"""
AGW 数据库迁移工具

支持: sqlite / mysql / postgresql 之间互转
流程: 读表结构 → 方言转译 → 建表 → 复制数据 → 修改 AGW 配置

用法:
  1. 编辑同目录下的 db_migrate.conf 配置源和目标数据库
  2. python3 db_migrate.py
  3. 确认后自动执行
  4. 重启 AGW 生效
"""

import os
import sys
import re
import configparser
from pathlib import Path

# ─── 默认配置 ─────────────────────────────────────────────────
CONFIG_DIR = Path(__file__).parent
CONFIG_FILE = CONFIG_DIR / "db_migrate.conf"

AGW_CONFIG_PATHS = [
    "/home/projects_docker/ai_gateway/dev/config/config.yaml",
    "/home/projects_docker/ai_gateway/prod/config/config.yaml",
]

DEFAULT_SOURCE = {
    "type": "sqlite",
    "path": "/home/projects_docker/ai_gateway/dev/data/agw.db",
}

DEFAULT_TARGET = {
    "type": "postgresql",
    "host": "127.0.0.1",
    "port": 5432,
    "user": "agw",
    "password": "",
    "database": "agw",
}

BATCH_SIZE = 1000
# ──────────────────────────────────────────────────────────────


# ═══════════════════════════════════════════════════════════════
# 方言转换器
# ═══════════════════════════════════════════════════════════════

# SQLite 亲和类型 → 标准类型
SQLITE_TYPE_MAP = {
    "INTEGER": "INTEGER",
    "INT": "INTEGER",
    "TEXT": "TEXT",
    "REAL": "REAL",
    "BLOB": "BLOB",
    "NUMERIC": "NUMERIC",
    "DATETIME": "DATETIME",
    "BOOLEAN": "BOOLEAN",
}

# → MySQL
def to_mysql_type(sqlite_type):
    t = sqlite_type.upper().split("(")[0]
    if t in ("INTEGER", "INT"):
        return "INT"
    if t == "TEXT":
        return "TEXT"
    if t == "REAL":
        return "DOUBLE"
    if t == "BLOB":
        return "BLOB"
    if t in ("DATETIME", "TIMESTAMP"):
        return "DATETIME"
    if t == "BOOLEAN":
        return "TINYINT(1)"
    return "TEXT"

# → PostgreSQL
def to_pg_type(sqlite_type):
    t = sqlite_type.upper().split("(")[0]
    if t in ("INTEGER", "INT"):
        return "INTEGER"
    if t == "TEXT":
        return "TEXT"
    if t == "REAL":
        return "DOUBLE PRECISION"
    if t == "BLOB":
        return "BYTEA"
    if t in ("DATETIME", "TIMESTAMP"):
        return "TIMESTAMP"
    if t == "BOOLEAN":
        return "BOOLEAN"
    return "TEXT"


def get_type_converter(tgt_type):
    if tgt_type == "mysql":
        return to_mysql_type
    elif tgt_type == "postgresql":
        return to_pg_type
    else:
        return lambda t: t.upper()  # SQLite → SQLite 保持原样


# ═══════════════════════════════════════════════════════════════
# 配置加载
# ═══════════════════════════════════════════════════════════════

def load_config():
    source = dict(DEFAULT_SOURCE)
    target = dict(DEFAULT_TARGET)
    if CONFIG_FILE.exists():
        cfg = configparser.ConfigParser()
        cfg.read(str(CONFIG_FILE))
        if "source" in cfg:
            for k in source:
                if k in cfg["source"]:
                    source[k] = cfg["source"][k]
        if "target" in cfg:
            for k in target:
                if k in cfg["target"]:
                    target[k] = cfg["target"][k]
    return source, target


def describe_db(cfg):
    t = cfg["type"]
    if t == "sqlite":
        return f"SQLite ({cfg['path']})"
    elif t == "mysql":
        return f"MySQL ({cfg['host']}:{cfg['port']}/{cfg['database']})"
    elif t == "postgresql":
        return f"PostgreSQL ({cfg['host']}:{cfg['port']}/{cfg['database']})"
    return str(t)


def find_agw_config():
    for path in AGW_CONFIG_PATHS:
        if os.path.exists(path):
            return path
    return None


# ═══════════════════════════════════════════════════════════════
# 数据库连接
# ═══════════════════════════════════════════════════════════════

def connect_source(cfg):
    t = cfg["type"]
    if t == "sqlite":
        import sqlite3
        path = cfg["path"]
        if not os.path.exists(path):
            raise FileNotFoundError(f"SQLite 文件不存在: {path}")
        # WAL checkpoint
        wal_conn = sqlite3.connect(path)
        wal_conn.execute("PRAGMA wal_checkpoint(TRUNCATE)")
        wal_conn.close()
        conn = sqlite3.connect(f"file:{path}?mode=ro", uri=True)
        conn.row_factory = sqlite3.Row
        return conn
    elif t == "mysql":
        try:
            import pymysql
        except ImportError:
            raise ImportError("需要 pymysql: pip install pymysql")
        return pymysql.connect(
            host=cfg["host"], port=int(cfg.get("port", 3306)),
            user=cfg["user"], password=cfg["password"],
            database=cfg["database"], charset="utf8mb4",
        )
    elif t == "postgresql":
        try:
            import psycopg2
            import psycopg2.extras
        except ImportError:
            raise ImportError("需要 psycopg2: pip install psycopg2-binary")
        return psycopg2.connect(
            host=cfg["host"], port=int(cfg.get("port", 5432)),
            user=cfg["user"], password=cfg["password"],
            dbname=cfg["database"],
        )
    else:
        raise ValueError(f"不支持的数据库类型: {t}")


def connect_target(cfg):
    t = cfg["type"]
    if t == "sqlite":
        import sqlite3
        path = cfg["path"]
        os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
        return sqlite3.connect(path)
    elif t == "mysql":
        try:
            import pymysql
        except ImportError:
            raise ImportError("需要 pymysql: pip install pymysql")
        return pymysql.connect(
            host=cfg["host"], port=int(cfg.get("port", 3306)),
            user=cfg["user"], password=cfg["password"],
            database=cfg["database"], charset="utf8mb4",
        )
    elif t == "postgresql":
        try:
            import psycopg2
            import psycopg2.extras
        except ImportError:
            raise ImportError("需要 psycopg2: pip install psycopg2-binary")
        conn = psycopg2.connect(
            host=cfg["host"], port=int(cfg.get("port", 5432)),
            user=cfg["user"], password=cfg["password"],
            dbname=cfg["database"],
        )
        conn.autocommit = True
        return conn
    else:
        raise ValueError(f"不支持的数据库类型: {t}")


# ═══════════════════════════════════════════════════════════════
# 表结构读取
# ═══════════════════════════════════════════════════════════════

def get_tables(conn, db_type):
    """获取所有用户表"""
    if db_type == "sqlite":
        cur = conn.execute(
            "SELECT name FROM sqlite_master "
            "WHERE type='table' AND name NOT LIKE 'sqlite_%'"
        )
        return sorted([r[0] for r in cur.fetchall()])
    elif db_type == "mysql":
        cur = conn.cursor()
        cur.execute("SHOW TABLES")
        return sorted([r[0] for r in cur.fetchall()])
    elif db_type == "postgresql":
        cur = conn.cursor()
        cur.execute(
            "SELECT table_name FROM information_schema.tables "
            "WHERE table_schema='public'"
        )
        return sorted([r[0] for r in cur.fetchall()])


def get_sqlite_table_info(conn, table):
    """
    获取 SQLite 表的完整结构信息
    返回: {
        "columns": [{"name": ..., "type": ..., "pk": ..., "notnull": ..., "default": ...}],
        "pk_columns": [...],
        "indexes": [{"name": ..., "unique": ..., "columns": [...]}],
    }
    """
    # 1. 列信息
    cur = conn.execute(f"PRAGMA table_info('{table}')")
    columns = []
    for r in cur:
        columns.append({
            "name": r[1],
            "type": r[2] or "TEXT",
            "notnull": bool(r[3]),
            "default": r[4],
            "pk": bool(r[5]),
        })
    
    pk_columns = [c["name"] for c in columns if c["pk"]]
    
    # 2. 索引
    cur = conn.execute(f"PRAGMA index_list('{table}')")
    indexes = []
    for idx in cur:
        seq, name, unique_flag = idx[0], idx[1], idx[2]
        # 排除主键索引和 sqlite_autoindex
        if name and not name.startswith("sqlite_autoindex"):
            cur2 = conn.execute(f"PRAGMA index_info('{name}')")
            idx_cols = [r[2] for r in cur2.fetchall()]
            if idx_cols:
                indexes.append({
                    "name": name,
                    "unique": bool(unique_flag),
                    "columns": idx_cols,
                })
    
    return {"columns": columns, "pk_columns": pk_columns, "indexes": indexes}


# ═══════════════════════════════════════════════════════════════
# DDL 生成
# ═══════════════════════════════════════════════════════════════

def generate_create_table(tgt_type, table, info):
    """根据表结构信息生成目标数据库的 CREATE TABLE 语句"""
    convert = get_type_converter(tgt_type)
    
    lines = []
    for col in info["columns"]:
        col_type = convert(col["type"])
        parts = [f'"{col["name"]}"', col_type]
        if col["notnull"]:
            parts.append("NOT NULL")
        if col["default"] is not None:
            d = col["default"]
            if d.upper() == "CURRENT_TIMESTAMP":
                if tgt_type == "postgresql":
                    parts.append("DEFAULT CURRENT_TIMESTAMP")
                else:
                    parts.append("DEFAULT CURRENT_TIMESTAMP")
            elif isinstance(d, str):
                parts.append(f"DEFAULT '{d}'")
            else:
                parts.append(f"DEFAULT {d}")
        lines.append("  " + " ".join(parts))
    
    # 主键
    if info["pk_columns"]:
        pk_str = ", ".join(f'"{c}"' for c in info["pk_columns"])
        if tgt_type == "sqlite":
            # SQLite 主键已在上面的列定义中处理（Pragma 已包含 pk 标志）
            # 但 AUTOINCREMENT 需要在建表语句中处理
            pass
        lines.append(f"  PRIMARY KEY ({pk_str})")
    
    ddl = f"CREATE TABLE IF NOT EXISTS \"{table}\" (\n" + ",\n".join(lines) + "\n)"
    
    if tgt_type == "mysql":
        ddl += " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"
    
    return ddl


def generate_create_indexes(tgt_type, table, info):
    """生成目标数据库的 CREATE INDEX 语句"""
    sqls = []
    for idx in info["indexes"]:
        unique_str = "UNIQUE " if idx["unique"] else ""
        cols_str = ", ".join(f'"{c}"' for c in idx["columns"])
        # 目标数据库的索引名称: table_idxname
        idx_name = f"{table}_{idx['name']}"
        sql = f"CREATE {unique_str}INDEX \"{idx_name}\" ON \"{table}\" ({cols_str})"
        sqls.append(sql)
    return sqls


def exec_ddl(tgt_conn, tgt_type, sql):
    """执行 DDL（建表/建索引），PG 需用 cursor"""
    if tgt_type == "postgresql":
        cur = tgt_conn.cursor()
        cur.execute(sql)
        cur.close()
    elif tgt_type == "mysql":
        cur = tgt_conn.cursor()
        cur.execute(sql)
        tgt_conn.commit()
    else:  # sqlite
        tgt_conn.execute(sql)
        tgt_conn.commit()

def quote_table(db_type, table):
    if db_type == "mysql":
        return f"`{table}`"
    return f'"{table}"'


def quote_cols(db_type, cols):
    names = [c["name"] if isinstance(c, dict) else c[0] for c in cols]
    if db_type == "mysql":
        return ", ".join(f"`{n}`" for n in names)
    return ", ".join(f'"{n}"' for n in names)


def get_row_count(conn, db_type, table):
    if db_type == "sqlite":
        cur = conn.execute(f'SELECT COUNT(*) FROM "{table}"')
    elif db_type == "mysql":
        cur = conn.cursor()
        cur.execute(f"SELECT COUNT(*) FROM `{table}`")
    elif db_type == "postgresql":
        cur = conn.cursor()
        cur.execute(f'SELECT COUNT(*) FROM "{table}"')
    return cur.fetchone()[0]


def migrate_data(conn, tgt_conn, db_type, tgt_type, table, columns):
    """迁移单表数据"""
    if isinstance(columns[0], dict):
        col_names = [c["name"] for c in columns]
    else:
        col_names = [c[0] for c in columns]
    
    total = get_row_count(conn, db_type, table)
    if total == 0:
        return 0
    
    tgt_cols = quote_cols(tgt_type, [{"name": n} for n in col_names])
    tgt_table = quote_table(tgt_type, table)
    src_table = quote_table(db_type, table)
    
    cur = conn.cursor()
    cur.execute(f"SELECT * FROM {src_table}")
    
    tgt_cur = tgt_conn.cursor()
    batch = []
    migrated = 0
    
    def flush():
        nonlocal migrated
        if not batch:
            return
        if tgt_type == "postgresql":
            import psycopg2.extras
            # execute_values 用单个 %s 占位整批值
            sql = f"INSERT INTO {tgt_table} ({tgt_cols}) VALUES %s"
            psycopg2.extras.execute_values(tgt_cur, sql, batch)
        elif tgt_type == "mysql":
            ph = ", ".join(["%s"] * len(col_names))
            sql = f"INSERT INTO {tgt_table} ({tgt_cols}) VALUES ({ph})"
            tgt_cur.executemany(sql, batch)
            tgt_conn.commit()
        else:  # sqlite
            ph = ", ".join(["?"] * len(col_names))
            sql = f"INSERT INTO {tgt_table} ({tgt_cols}) VALUES ({ph})"
            tgt_cur.executemany(sql, batch)
            tgt_conn.commit()
        migrated += len(batch)
        batch.clear()
    
    for row in cur:
        batch.append(tuple(row))
        if len(batch) >= BATCH_SIZE:
            flush()
            print(f"    ... {migrated}/{total}", end="\r")
    
    flush()
    return migrated


# ═══════════════════════════════════════════════════════════════
# AGW 配置修改
# ═══════════════════════════════════════════════════════════════

def update_agw_config(target):
    path = find_agw_config()
    if not path:
        print("  ⚠️  未找到 AGW 配置文件，请手动修改 db.type")
        return False
    
    import yaml
    
    with open(path, "r") as f:
        config = yaml.safe_load(f)
    
    t = target["type"]
    config["db"]["type"] = t
    
    if t == "sqlite":
        config["db"]["path"] = target.get("path", "data/agw.db")
    else:
        config["db"]["host"] = target["host"]
        config["db"]["port"] = int(target["port"])
        config["db"]["user"] = target["user"]
        config["db"]["password"] = target["password"]
        config["db"]["name"] = target["database"]
    
    with open(path, "w") as f:
        yaml.safe_dump(config, f, default_flow_style=False, allow_unicode=True)
    
    print(f"  ✅ 配置文件已更新: {path}")
    return True


# ═══════════════════════════════════════════════════════════════
# 主流程
# ═══════════════════════════════════════════════════════════════

def main():
    print()
    print("=" * 50)
    print("  AGW 数据库迁移工具")
    print("=" * 50)
    print()
    
    source, target = load_config()
    db_type, tgt_type = source["type"], target["type"]
    
    print(f"  迁移方案：")
    print(f"    {describe_db(source)}  →  {describe_db(target)}")
    print()
    print(f"  将自动完成: 读表结构 → 方言转译 → 建表 → 复制数据")
    print(f"  源数据库只读，不会被修改。")
    print()
    
    confirm = input("  确认执行？(yes/no): ").strip().lower()
    if confirm not in ("yes", "y"):
        print("  已取消。")
        return
    
    # ── 连接 ──
    print()
    print("  [1/4] 连接源数据库...", end=" ")
    try:
        conn = connect_source(source)
        print("✅")
    except Exception as e:
        print(f"❌\n  {e}")
        sys.exit(1)
    
    print("  [2/4] 连接目标数据库...", end=" ")
    try:
        tgt_conn = connect_target(target)
        print("✅")
    except Exception as e:
        print(f"❌\n  {e}")
        conn.close()
        sys.exit(1)
    
    # ── 读表结构 & 建表 ──
    print("  [3/4] 读表结构 & 建表...")
    tables = get_tables(conn, db_type)
    
    for table in tables:
        if db_type == "sqlite":
            info = get_sqlite_table_info(conn, table)
        else:
            # MySQL/PG 源：用 information_schema 读列信息，暂时简化
            info = {"columns": [], "pk_columns": [], "indexes": []}
            cur = conn.cursor()
            if db_type == "mysql":
                cur.execute(f"DESCRIBE `{table}`")
                for r in cur:
                    info["columns"].append({
                        "name": r[0], "type": r[1],
                        "notnull": r[2] == "NO", "default": r[4],
                        "pk": r[3] == "PRI",
                    })
            elif db_type == "postgresql":
                cur.execute(
                    "SELECT column_name, data_type, is_nullable, column_default "
                    "FROM information_schema.columns "
                    "WHERE table_schema='public' AND table_name=%s "
                    "ORDER BY ordinal_position", (table,)
                )
                for r in cur:
                    info["columns"].append({
                        "name": r[0], "type": r[1],
                        "notnull": r[2] == "NO", "default": r[3],
                        "pk": False,
                    })
            info["pk_columns"] = [c["name"] for c in info["columns"] if c["pk"]]
        
        # 生成并执行 CREATE TABLE
        ddl = generate_create_table(tgt_type, table, info)
        try:
            exec_ddl(tgt_conn, tgt_type, ddl)
            print(f"    - 建表: {table} ✅")
        except Exception as e:
            print(f"    - 建表: {table} ❌ ({e})")
            continue
        
        # 生成并执行 CREATE INDEX
        if "indexes" in info:
            for idx_sql in generate_create_indexes(tgt_type, table, info):
                try:
                    exec_ddl(tgt_conn, tgt_type, idx_sql)
                except Exception:
                    pass  # 索引可能已存在
    
    print()
    
    # ── 复制数据 ──
    print("  [4/4] 复制数据...")
    total_rows = 0
    ok, failed = [], []
    
    for table in tables:
        try:
            if db_type == "sqlite":
                cols = [{"name": c["name"], "type": c["type"]} for c in get_sqlite_table_info(conn, table)["columns"]]
            else:
                # 从已获取的 info 取列
                cols = info["columns"]
            rows = migrate_data(conn, tgt_conn, db_type, tgt_type, table, cols)
            print(f"    - {table} ({rows} 行) ✅")
            total_rows += rows
            ok.append(table)
        except Exception as e:
            print(f"    - {table} ❌ ({e})")
            failed.append(table)
    
    print()
    print(f"    成功: {len(ok)} 张表, {total_rows} 行")
    if failed:
        print(f"    失败: {len(failed)} 张表 ({', '.join(failed)})")
    print()
    
    conn.close()
    tgt_conn.close()
    
    # ── 改配置 ──
    if failed:
        print(f"  ⚠️  有 {len(failed)} 张表失败，不修改 AGW 配置。")
        print(f"  请排查后重新运行。")
        print()
        print("=" * 50)
        print("  迁移未完全成功，请检查上述错误。")
        print("=" * 50)
        print()
        return
    
    print("  正在修改 AGW 配置...")
    update_agw_config(target)
    
    print()
    print("=" * 50)
    print("  迁移完成！请重启 AGW 使新配置生效。")
    print("=" * 50)
    print()


if __name__ == "__main__":
    main()