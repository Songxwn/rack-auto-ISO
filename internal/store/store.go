package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Songxwn/rack-auto-ISO/internal/model"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	dir string
	mu  sync.RWMutex
	st  model.State
}

func Open(dir string) (*Store, error) {
	s := &Store{dir: dir}
	path := filepath.Join(dir, "state.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		s.st = defaultState()
		if err := s.persistLocked(); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &s.st); err != nil {
		return nil, fmt.Errorf("parse state.json: %w", err)
	}
	if s.st.Settings.DefaultNet.Mode == "" {
		s.st.Settings.DefaultNet.Mode = model.NetDHCP
	}
	if s.st.Settings.ISONet.Mode == "" {
		s.st.Settings.ISONet.Mode = model.NetDHCP
	}
	if len(s.st.Menus) == 0 {
		s.st.Menus = []model.Menu{defaultMenu()}
	}
	return s, nil
}

func defaultState() model.State {
	now := time.Now().UTC()
	return model.State{
		Settings: model.Settings{
			ServerName: "rack-auto-ISO",
			PublicURL:  "",
			DefaultNet: model.NetworkConfig{Mode: model.NetDHCP},
			ISONet:     model.NetworkConfig{Mode: model.NetDHCP},
			UpdatedAt:  now,
		},
		Menus: []model.Menu{defaultMenu()},
		ISOs:  []model.ISOFile{},
	}
}

func defaultMenu() model.Menu {
	now := time.Now().UTC()
	return model.Menu{
		ID:         "default",
		Name:       "默认启动菜单",
		Description: "机架自动装机菜单",
		TimeoutSec: 30,
		Items: []model.MenuItem{
			{ID: "shell", Label: "iPXE Shell", Type: model.ItemShell, Enabled: true},
			{ID: "exit", Label: "退出 / 继续 BIOS", Type: model.ItemExit, Enabled: true},
		},
		UpdatedAt: now,
	}
}

func (s *Store) DataDir() string { return s.dir }
func (s *Store) ISODir() string  { return filepath.Join(s.dir, "isos") }

func (s *Store) persistLocked() error {
	path := filepath.Join(s.dir, "state.json")
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(s.st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) Snapshot() model.State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneState(s.st)
}

func (s *Store) Settings() model.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.st.Settings
}

func (s *Store) SaveSettings(cfg model.Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg.UpdatedAt = time.Now().UTC()
	s.st.Settings = cfg
	return s.persistLocked()
}

func (s *Store) ListMenus() []model.Menu {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Menu, len(s.st.Menus))
	copy(out, s.st.Menus)
	return out
}

func (s *Store) GetMenu(id string) (model.Menu, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.st.Menus {
		if m.ID == id {
			return m, nil
		}
	}
	return model.Menu{}, ErrNotFound
}

func (s *Store) UpsertMenu(m model.Menu) (model.Menu, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	m.UpdatedAt = time.Now().UTC()
	for i := range m.Items {
		if m.Items[i].ID == "" {
			m.Items[i].ID = uuid.NewString()
		}
	}
	found := false
	for i, existing := range s.st.Menus {
		if existing.ID == m.ID {
			s.st.Menus[i] = m
			found = true
			break
		}
	}
	if !found {
		s.st.Menus = append(s.st.Menus, m)
	}
	if err := s.persistLocked(); err != nil {
		return model.Menu{}, err
	}
	return m, nil
}

func (s *Store) DeleteMenu(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "default" {
		return fmt.Errorf("cannot delete default menu")
	}
	out := s.st.Menus[:0]
	found := false
	for _, m := range s.st.Menus {
		if m.ID == id {
			found = true
			continue
		}
		out = append(out, m)
	}
	if !found {
		return ErrNotFound
	}
	s.st.Menus = out
	return s.persistLocked()
}

func (s *Store) ListISOs() []model.ISOFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.ISOFile, len(s.st.ISOs))
	copy(out, s.st.ISOs)
	return out
}

func (s *Store) GetISO(id string) (model.ISOFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, f := range s.st.ISOs {
		if f.ID == id {
			return f, nil
		}
	}
	return model.ISOFile{}, ErrNotFound
}

func (s *Store) AddISO(meta model.ISOFile) (model.ISOFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if meta.ID == "" {
		meta.ID = uuid.NewString()
	}
	meta.UploadedAt = time.Now().UTC()
	s.st.ISOs = append(s.st.ISOs, meta)
	if err := s.persistLocked(); err != nil {
		return model.ISOFile{}, err
	}
	return meta, nil
}

func (s *Store) DeleteISO(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var target *model.ISOFile
	out := s.st.ISOs[:0]
	for i := range s.st.ISOs {
		f := s.st.ISOs[i]
		if f.ID == id {
			cp := f
			target = &cp
			continue
		}
		out = append(out, f)
	}
	if target == nil {
		return ErrNotFound
	}
	s.st.ISOs = out
	if err := s.persistLocked(); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(s.ISODir(), target.Filename))
	return nil
}

func (s *Store) ISOPath(filename string) string {
	return filepath.Join(s.ISODir(), filename)
}

func cloneState(in model.State) model.State {
	out := in
	out.Menus = append([]model.Menu(nil), in.Menus...)
	out.ISOs = append([]model.ISOFile(nil), in.ISOs...)
	return out
}
