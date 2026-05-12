package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	sqlitedrv "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// VersionFilePath 版本标记文件路径
const VersionFilePath = "data/.agw_version"

// BackupDir 数据库备份目录
const BackupDir = "data/backups"

// BackupRetentionDays 旧备份保留天数
const BackupRetentionDays = 30

// CurrentVersion 当前程序版本（与 docs/VERSION 保持同步）
const CurrentVersion = "0.2.0"

// RunMigration 启动时统一迁移入口
// 流程：检查版本标记 → 如果版本匹配则跳过 → 否则备份DB → 执行迁移 → 写入版本标记
// 注意：
//   - 配置补全（EnsureConfigCompleteness）在 config.Load() 中完成（需要 viper 上下文）
//   - DB 表 AutoMigrate 由各自包的初始化函数完成，不在本文件中集中管理
func RunMigration(configPath string, db *gorm.DB, logger *zap.Logger) error {
	// 1. 清理版本文件中的 FAILED 标记（上次迁移失败残留）
	_ = cleanupFailedMark(logger)

	// 2. 版本检测
	needsMigration, err := checkVersion(logger)
	if err != nil {
		if logger != nil {
			logger.Warn("check version failed, will run migration anyway", zap.Error(err))
		}
		needsMigration = true
	}
	if !needsMigration {
		if logger != nil {
			logger.Info("version match, skip migration", zap.String("version", CurrentVersion))
		}
		// 清理超期旧备份
		cleanupOldBackups(logger)
		return nil
	}

	if logger != nil {
		logger.Info("running migration", zap.String("version", CurrentVersion))
	}

	// 3. 写入 MIGRATING 状态标记
	if err := writeVersionState("MIGRATING", logger); err != nil {
		if logger != nil {
			logger.Warn("write MIGRATING state failed, continuing", zap.Error(err))
		}
	}

	// 4. 备份数据库
	dbPath := "data/agw.db"
	backupPath, backupErr := backupDatabase(dbPath, logger)
	if backupErr != nil {
		if logger != nil {
			logger.Warn("database backup failed, continuing without backup", zap.Error(backupErr))
		}
	}

	// 5. 兼容旧环境变量迁移
	migrateEnvVariables("./config/.env", logger)

	// 6. 写入版本标记（OK 状态）
	if err := writeVersionState("OK "+CurrentVersion, logger); err != nil {
		// 写入失败 → 尝试恢复备份
		if backupPath != "" {
			if logger != nil {
				logger.Error("version write failed, attempting restore from backup", zap.Error(err))
			}
			if restoreErr := restoreDatabase(dbPath, backupPath, logger); restoreErr != nil {
				if logger != nil {
					logger.Error("restore failed", zap.Error(restoreErr))
				}
			}
			// 标记 FAILED
			_ = writeVersionState("FAILED", logger)
			return fmt.Errorf("migration failed (restored from backup): %w", err)
		}
		_ = writeVersionState("FAILED", logger)
		return fmt.Errorf("migration failed: %w", err)
	}

	// 7. 清理超期旧备份
	cleanupOldBackups(logger)

	if logger != nil {
		logger.Info("migration completed", zap.String("version", CurrentVersion))
		if backupPath != "" {
			logger.Info("backup saved", zap.String("path", backupPath))
		}
	}
	return nil
}

// checkVersion 检查版本标记文件，返回是否需要执行迁移
func checkVersion(logger *zap.Logger) (bool, error) {
	data, err := os.ReadFile(VersionFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			if logger != nil {
				logger.Debug("version file not found, migration needed")
			}
			return true, nil
		}
		return true, err
	}
	version := strings.TrimSpace(string(data))

	// FAILED 标记 → 需要重新迁移
	if version == "FAILED" || strings.HasPrefix(version, "FAILED") {
		if logger != nil {
			logger.Warn("previous migration failed, retrying")
		}
		return true, nil
	}

	// MIGRATING 残留 → 上次异常中断，需要重新迁移
	if version == "MIGRATING" {
		if logger != nil {
			logger.Warn("previous migration interrupted, retrying")
		}
		return true, nil
	}

	// 提取版本号：格式 "OK 0.2.0" 或 "0.2.0"
	ver := strings.TrimPrefix(version, "OK ")
	return ver != CurrentVersion, nil
}

// writeVersionState 写入带状态的版本标记
func writeVersionState(state string, logger *zap.Logger) error {
	dir := filepath.Dir(VersionFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create version dir: %w", err)
	}
	if err := os.WriteFile(VersionFilePath, []byte(state+"\n"), 0644); err != nil {
		return fmt.Errorf("write version file: %w", err)
	}
	if logger != nil {
		logger.Debug("version state written", zap.String("state", state))
	}
	return nil
}

// cleanupFailedMark 清理上次迁移失败的标记文件
func cleanupFailedMark(logger *zap.Logger) error {
	data, err := os.ReadFile(VersionFilePath)
	if err != nil {
		return nil // 文件不存在，无操作
	}
	version := strings.TrimSpace(string(data))
	if version == "FAILED" {
		if logger != nil {
			logger.Warn("cleaning up previous FAILED migration mark")
		}
		return os.Remove(VersionFilePath)
	}
	return nil
}

// backupDatabase 备份数据库文件
// WAL 模式下先执行 checkpoint 合并数据到主文件，再备份
// 返回备份文件路径，失败时返回空字符串和错误
func backupDatabase(dbPath string, logger *zap.Logger) (string, error) {
	// 检查源文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if logger != nil {
			logger.Debug("no existing database to backup (first run)")
		}
		return "", nil
	}

	// WAL checkpoint：合并所有 WAL 数据到主文件
	if err := walCheckpoint(dbPath); err != nil {
		if logger != nil {
			logger.Warn("WAL checkpoint failed, continuing backup anyway", zap.Error(err))
		}
	}

	// 确保备份目录存在
	if err := os.MkdirAll(BackupDir, 0755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	// 生成备份文件名
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("agw.db.backup.v0.1.5_%s.db", timestamp)
	backupPath := filepath.Join(BackupDir, backupName)

	// 复制主文件
	src, err := os.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("open source db: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(backupPath)
		return "", fmt.Errorf("copy db: %w", err)
	}

	if logger != nil {
		logger.Info("database backup created", zap.String("path", backupPath))
	}
	return backupPath, nil
}

// walCheckpoint 对 SQLite 数据库执行 WAL checkpoint
// 将所有 WAL 数据合并到主文件（TRUNCATE 模式：合并后删除 WAL 文件）
func walCheckpoint(dbPath string) error {
	db, err := gorm.Open(sqlitedrv.Open(dbPath+"?_journal_mode=WAL"), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open db for checkpoint: %w", err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}

	// PRAGMA wal_checkpoint(TRUNCATE) — 成功后 WAL 文件被删除
	var result []map[string]interface{}
	rows, err := sqlDB.Query("PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		return fmt.Errorf("wal_checkpoint query: %w", err)
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}
		row := make(map[string]interface{})
		for i, col := range cols {
			row[col] = values[i]
		}
		result = append(result, row)
	}

	return nil
}

// restoreDatabase 从备份恢复数据库
func restoreDatabase(dbPath, backupPath string, logger *zap.Logger) error {
	if logger != nil {
		logger.Warn("restoring database from backup", zap.String("backup", backupPath))
	}

	// 先写入临时文件，校验通过后再替换
	tmpPath := dbPath + ".restore.tmp"

	src, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create restore tmp: %w", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("copy backup: %w", err)
	}
	dst.Close()

	// 校验临时文件大小与备份一致
	tmpInfo, _ := os.Stat(tmpPath)
	srcInfo, _ := os.Stat(backupPath)
	if tmpInfo.Size() != srcInfo.Size() {
		os.Remove(tmpPath)
		return fmt.Errorf("restore verification failed: size mismatch")
	}

	// 替换
	if err := os.Rename(tmpPath, dbPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace db with backup: %w", err)
	}

	if logger != nil {
		logger.Info("database restored successfully")
	}
	return nil
}

// cleanupOldBackups 清理超过保留天数的旧备份
func cleanupOldBackups(logger *zap.Logger) {
	entries, err := os.ReadDir(BackupDir)
	if err != nil {
		return // 备份目录不存在，跳过
	}

	cutoff := time.Now().AddDate(0, 0, -BackupRetentionDays)
	deleted := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "agw.db.backup.") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(BackupDir, entry.Name())
			if err := os.Remove(path); err == nil {
				deleted++
			}
		}
	}

	if deleted > 0 && logger != nil {
		logger.Info("cleaned old backups", zap.Int("count", deleted))
	}
}

// writeVersionFile 写入版本标记文件（兼容旧版调用）
func writeVersionFile(logger *zap.Logger) error {
	return writeVersionState("OK "+CurrentVersion, logger)
}

// migrateEnvVariables 环境变量自动迁移：旧 AGW_SERVER_API_TOKEN → 新 AGW_ADMIN_USER/PASS
func migrateEnvVariables(envPath string, logger *zap.Logger) {
	adminUser := os.Getenv("AGW_ADMIN_USER")
	adminPass := os.Getenv("AGW_ADMIN_PASS")

	if adminUser == "" || adminPass == "" {
		oldToken := os.Getenv("AGW_SERVER_API_TOKEN")
		if oldToken == "" {
			return
		}

		if logger != nil {
			logger.Warn("==============================================")
			logger.Warn("  v0.2.0 BREAKING CHANGE")
			logger.Warn("  AGW_SERVER_API_TOKEN 已废弃，正在自动迁移...")
			logger.Warn("==============================================")
		}

		// 自动写入 AGW_ADMIN_USER/PASS 并删除旧的 AGW_SERVER_API_TOKEN
		if err := migrateEnvFile(envPath, oldToken, logger); err != nil {
			if logger != nil {
				logger.Warn("自动迁移 .env 失败，请手动添加以下变量：", zap.Error(err))
				logger.Warn("  AGW_ADMIN_USER=admin")
				logger.Warn("  AGW_ADMIN_PASS=<你的密码>")
				logger.Warn("  并删除 AGW_SERVER_API_TOKEN 行")
			}
		} else {
			// 设置新环境变量供后续使用
			os.Setenv("AGW_ADMIN_USER", "admin")
			os.Setenv("AGW_ADMIN_PASS", oldToken)
			os.Unsetenv("AGW_SERVER_API_TOKEN")

			if logger != nil {
				logger.Info("✅ 自动迁移完成：AGW_SERVER_API_TOKEN → AGW_ADMIN_USER/PASS")
			}
		}
	}
}

// migrateEnvFile 修改 .env 文件：添加 AGW_ADMIN_USER/PASS，删除 AGW_SERVER_API_TOKEN 行
func migrateEnvFile(envPath, oldToken string, logger *zap.Logger) error {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}

	content := string(data)
	oldTokenLine := "AGW_SERVER_API_TOKEN=" + oldToken

	// 替换：旧的 AGW_SERVER_API_TOKEN 行 → 新变量
	replacement := "AGW_ADMIN_USER=admin\nAGW_ADMIN_PASS=" + oldToken
	if !strings.Contains(content, oldTokenLine) {
		// 兼容：值可能不同（如换行差异），尝试以 AGW_SERVER_API_TOKEN= 开头的行
		return replaceEnvLine(&content, "AGW_SERVER_API_TOKEN=", replacement)
	}

	content = strings.Replace(content, oldTokenLine, replacement, 1)
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		return err
	}
	return nil
}

// replaceEnvLine 在 content 中查找并替换以 prefix 开头的行
func replaceEnvLine(content *string, prefix, replacement string) error {
	lines := strings.Split(*content, "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			lines[i] = replacement
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("未找到 %s 开头的行", prefix)
	}
	*content = strings.Join(lines, "\n")
	return nil
}