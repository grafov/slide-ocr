package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const profileWord = "Профиль"

var (
	errLastProfile    = errors.New("cannot delete the last profile")
	errUnknownProfile = errors.New("unknown profile")
)

type backendProfile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	APIKey    string `json:"apiKey"`
	Model     string `json:"model"`
	Reasoning bool   `json:"reasoning"`
	Effort    string `json:"effort"`
	Temp      string `json:"temp"`
	Tokens    string `json:"tokens"`
}

type profileSet struct {
	Active string           `json:"active"`
	Items  []backendProfile `json:"items"`
}

type legacyBackend struct {
	URL       string
	APIKey    string
	Model     string
	Reasoning bool
	Effort    string
	Temp      string
	Tokens    string
}

func profilesFromStored(raw string, legacy legacyBackend) profileSet {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		var stored profileSet
		if err := json.Unmarshal([]byte(raw), &stored); err == nil && len(stored.Items) > 0 {
			return normalizeProfiles(stored)
		}
	}

	p := fillProfileDefaults(backendProfile{
		ID:        "p1",
		Name:      profileTitle(1),
		URL:       legacy.URL,
		APIKey:    legacy.APIKey,
		Model:     legacy.Model,
		Reasoning: legacy.Reasoning,
		Effort:    legacy.Effort,
		Temp:      legacy.Temp,
		Tokens:    legacy.Tokens,
	})

	return profileSet{Active: p.ID, Items: []backendProfile{p}}
}

func marshalProfiles(s profileSet) (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func appendProfile(s profileSet) profileSet {
	if len(s.Items) == 0 {
		p := fillProfileDefaults(backendProfile{ID: "p1", Name: profileTitle(1)})
		return profileSet{Active: p.ID, Items: []backendProfile{p}}
	}

	idx := profileIndex(s, s.Active)
	if idx < 0 {
		idx = 0
		s.Active = s.Items[0].ID
	}

	cp := s.Items[idx]
	cp.ID = freshProfileID(profileIDs(s.Items))
	cp.Name = nextProfileName(s.Items)
	s.Items = append(s.Items, cp)
	s.Active = cp.ID

	return s
}

func removeProfile(s profileSet, id string) (profileSet, error) {
	if len(s.Items) <= 1 {
		return s, errLastProfile
	}

	idx := profileIndex(s, id)
	if idx < 0 {
		return s, errUnknownProfile
	}

	s.Items = append(s.Items[:idx], s.Items[idx+1:]...)
	if s.Active == id {
		neighbor := idx - 1
		if neighbor < 0 {
			neighbor = 0
		}
		s.Active = s.Items[neighbor].ID
	}

	return s, nil
}

func selectProfile(s profileSet, id string) (profileSet, error) {
	if profileIndex(s, id) < 0 {
		return s, errUnknownProfile
	}

	s.Active = id

	return s, nil
}

func profileIndex(s profileSet, id string) int {
	for i := range s.Items {
		if s.Items[i].ID == id {
			return i
		}
	}

	return -1
}

func profileLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return profileWord
	}

	return name
}

func normalizeProfiles(s profileSet) profileSet {
	used := make(map[string]struct{}, len(s.Items))
	for i := range s.Items {
		id := s.Items[i].ID
		if _, exists := used[id]; id == "" || exists {
			id = freshProfileID(used)
		}
		s.Items[i].ID = id
		used[id] = struct{}{}
		s.Items[i] = fillProfileDefaults(s.Items[i])
	}
	if profileIndex(s, s.Active) < 0 {
		s.Active = s.Items[0].ID
	}

	return s
}

func fillProfileDefaults(p backendProfile) backendProfile {
	if strings.TrimSpace(p.URL) == "" {
		p.URL = defaultURL
	}
	switch p.Effort {
	case effortLow, effortMedium, effortHigh:
	default:
		p.Effort = defaultEffort
	}
	if strings.TrimSpace(p.Temp) == "" {
		p.Temp = defaultTemp
	}
	if strings.TrimSpace(p.Tokens) == "" {
		p.Tokens = defaultTokens
	}

	return p
}

func nextProfileName(items []backendProfile) string {
	maxN := len(items)
	prefix := profileWord + " "
	for _, p := range items {
		rest, ok := strings.CutPrefix(strings.TrimSpace(p.Name), prefix)
		if !ok {
			continue
		}
		n, err := strconv.Atoi(rest)
		if err != nil {
			continue
		}
		if n > maxN {
			maxN = n
		}
	}

	return profileTitle(maxN + 1)
}

func profileTitle(n int) string {
	return fmt.Sprintf("%s %d", profileWord, n)
}

func profileIDs(items []backendProfile) map[string]struct{} {
	used := make(map[string]struct{}, len(items))
	for _, p := range items {
		if p.ID != "" {
			used[p.ID] = struct{}{}
		}
	}

	return used
}

func freshProfileID(used map[string]struct{}) string {
	for n := 1; ; n++ {
		id := fmt.Sprintf("p%d", n)
		if _, ok := used[id]; !ok {
			return id
		}
	}
}
