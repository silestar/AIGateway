package i18n

// i18n 加载器桩
// 阶段一仅定义结构，后续实现完整的多语言支持

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// I18n 国际化服务
type I18n struct {
	mu      sync.RWMutex
	locales map[string]map[string]string
	lang    string
}

// New 创建 i18n 实例
func New(dir, defaultLang string) (*I18n, error) {
	i := &I18n{
		locales: make(map[string]map[string]string),
		lang:    defaultLang,
	}
	if err := i.loadDir(dir); err != nil {
		return nil, err
	}
	return i, nil
}

// T 翻译
func (i *I18n) T(key string) string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if m, ok := i.locales[i.lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return key
}

// SetLang 切换语言
func (i *I18n) SetLang(lang string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.lang = lang
}

func (i *I18n) loadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		lang := entry.Name()[:len(entry.Name())-5] // 去掉 .json
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		m := make(map[string]string)
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		i.locales[lang] = m
	}
	return nil
}
