# AGW 数据库迁移工具

## 用途

在 SQLite / MySQL / PostgreSQL 之间迁移 AGW 数据库，**一步完成**：读表结构 → 方言转译 → 建表 → 复制数据 → 修改配置。

## 支持的方向

| 源 → 目标 | sqlite | mysql | postgresql |
|-----------|--------|-------|------------|
| sqlite | ✅ | ✅ | ✅ |
| mysql | ✅ | ✅ | ✅ |
| postgresql | ✅ | ✅ | ✅ |

## 使用步骤

### 1. 配置

```bash
cp db_migrate.conf.example db_migrate.conf
# 编辑 db_migrate.conf，填写源和目标数据库信息
```

### 2. 确保目标 PostgreSQL 用户有建表权限（仅 PG 目标需要）

```sql
GRANT ALL ON SCHEMA public TO agw;
```

### 3. 运行

```bash
python3 db_migrate.py
```

脚本会展示迁移方案 → 确认 → 自动执行。

### 4. 重启 AGW

迁移完成后重启 AGW 容器使新配置生效。

## 依赖

- Python 3.x 标准库
- 目标为 MySQL 时需 `pymysql`：`pip install pymysql`
- 目标为 PostgreSQL 时需 `psycopg2`：`pip install psycopg2-binary`
- 脚本采用懒加载：不用的驱动不需要安装

## 流程

```
[1/4] 连接源数据库（SQLite 自动 WAL checkpoint + 只读模式）
[2/4] 连接目标数据库
[3/4] 读表结构 & 建表
      ├─ PRAGMA table_info / index_list
      ├─ 方言转译（TEXT→TEXT, INTEGER→INTEGER, BLOB→BYTEA...）
      ├─ CREATE TABLE IF NOT EXISTS（列定义 + NOT NULL + DEFAULT + 主键）
      └─ CREATE INDEX（含 UNIQUE）
[4/4] 复制数据（逐表分批 1000 行）
→ 全表成功 → 修改 config.yaml
→ 有失败 → 不改配置，源库无损
```

## 安全

- 源数据库**只读**，不做任何修改
- 任何失败都不会修改 AGW 配置
- 源库完好无损，改回配置重启即可回滚

## 配置文件示例

```ini
[source]
type = sqlite
path = /home/projects_docker/ai_gateway/dev/data/agw.db

[target]
type = postgresql
host = 127.0.0.1
port = 5432
user = agw
password = 
database = agw
```