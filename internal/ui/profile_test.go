package ui

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("profilesFromStored", func() {
	It("builds the first profile from legacy backend settings", func() {
		got := profilesFromStored("", legacyBackend{
			URL:       "http://x",
			APIKey:    "k",
			Model:     "m",
			Reasoning: true,
			Effort:    effortHigh,
			Temp:      "0.9",
			Tokens:    "100",
		})

		Expect(got.Active).To(Equal("p1"))
		Expect(got.Items).To(HaveLen(1))
		Expect(got.Items[0]).To(Equal(backendProfile{
			ID:        "p1",
			Name:      "Профиль 1",
			URL:       "http://x",
			APIKey:    "k",
			Model:     "m",
			Reasoning: true,
			Effort:    effortHigh,
			Temp:      "0.9",
			Tokens:    "100",
		}))
	})

	It("fills defaults when legacy settings are empty", func() {
		got := profilesFromStored("  ", legacyBackend{})

		Expect(got.Items[0].URL).To(Equal(defaultURL))
		Expect(got.Items[0].Effort).To(Equal(defaultEffort))
		Expect(got.Items[0].Temp).To(Equal(defaultTemp))
		Expect(got.Items[0].Tokens).To(Equal(defaultTokens))
		Expect(got.Items[0].Reasoning).To(BeFalse())
		Expect(got.Items[0].Name).To(Equal("Профиль 1"))
	})

	It("keeps stored profiles and ignores legacy values", func() {
		stored := profileSet{
			Active: "p2",
			Items: []backendProfile{
				{ID: "p1", Name: "local", URL: "http://local", Effort: effortLow, Temp: "0.1", Tokens: "8"},
				{ID: "p2", Name: "remote", URL: "http://remote", APIKey: "secret", Model: "vision", Reasoning: true, Effort: effortHigh, Temp: "0.4", Tokens: "2048"},
			},
		}
		raw, err := marshalProfiles(stored)
		Expect(err).NotTo(HaveOccurred())

		got := profilesFromStored(raw, legacyBackend{URL: "http://ignored", Model: "old"})

		Expect(got.Active).To(Equal("p2"))
		Expect(got.Items).To(Equal(stored.Items))
	})

	It("falls back to legacy settings when the stored json is invalid", func() {
		got := profilesFromStored("{", legacyBackend{URL: "http://legacy", Model: "m"})

		Expect(got.Items).To(HaveLen(1))
		Expect(got.Items[0].URL).To(Equal("http://legacy"))
		Expect(got.Items[0].Model).To(Equal("m"))
		Expect(got.Items[0].Name).To(Equal("Профиль 1"))
	})

	It("repairs a missing active id and fills empty provider fields", func() {
		raw := `{"active":"missing","items":[{"id":"p9","name":"Z","url":"","effort":"nope"}]}`
		got := profilesFromStored(raw, legacyBackend{URL: "http://legacy"})

		Expect(got.Active).To(Equal("p9"))
		Expect(got.Items[0].URL).To(Equal(defaultURL))
		Expect(got.Items[0].Effort).To(Equal(defaultEffort))
		Expect(got.Items[0].Temp).To(Equal(defaultTemp))
		Expect(got.Items[0].Tokens).To(Equal(defaultTokens))
		Expect(got.Items[0].Name).To(Equal("Z"))
	})

	It("replaces duplicate ids and keeps an empty name", func() {
		raw := `{"active":"p1","items":[{"id":"p1","name":"A","url":"http://a","effort":"low","temp":"0.2","tokens":"1"},{"id":"p1","name":"","url":"http://b","effort":"low","temp":"0.2","tokens":"1"}]}`
		got := profilesFromStored(raw, legacyBackend{})

		Expect(got.Items[0].ID).To(Equal("p1"))
		Expect(got.Items[1].ID).To(Equal("p2"))
		Expect(got.Items[1].Name).To(BeEmpty())
		Expect(profileLabel(got.Items[1].Name)).To(Equal(profileWord))
		Expect(got.Active).To(Equal("p1"))
	})
})

var _ = Describe("profile list operations", func() {
	It("round-trips profiles through json", func() {
		in := profileSet{
			Active: "p1",
			Items: []backendProfile{{
				ID: "p1", Name: "Профиль 1", URL: defaultURL, APIKey: "k", Model: "m",
				Reasoning: true, Effort: effortMedium, Temp: defaultTemp, Tokens: defaultTokens,
			}},
		}
		raw, err := marshalProfiles(in)
		Expect(err).NotTo(HaveOccurred())

		var got profileSet
		Expect(json.Unmarshal([]byte(raw), &got)).To(Succeed())
		Expect(got).To(Equal(in))
	})

	It("copies the active profile under the next name and selects it", func() {
		s := profilesFromStored("", legacyBackend{URL: "http://a", Model: "m1", Effort: effortLow, Temp: "0.5", Tokens: "9"})
		s.Items[0].Name = "LM Studio"
		s = appendProfile(s)
		s = appendProfile(s)

		Expect(s.Items).To(HaveLen(3))
		Expect(s.Items[0].Name).To(Equal("LM Studio"))
		Expect(s.Items[1].Name).To(Equal("Профиль 2"))
		Expect(s.Items[2].Name).To(Equal("Профиль 3"))
		Expect(s.Items[1].URL).To(Equal("http://a"))
		Expect(s.Items[2].Model).To(Equal("m1"))
		Expect(s.Items[2].Effort).To(Equal(effortLow))
		Expect(s.Active).To(Equal(s.Items[2].ID))
		Expect(s.Items[2].ID).NotTo(Equal(s.Items[1].ID))
	})

	It("numbers a new name above an existing Профиль prefix", func() {
		s := profileSet{
			Active: "p1",
			Items:  []backendProfile{{ID: "p1", Name: "Профиль 5", URL: defaultURL, Effort: effortMedium, Temp: defaultTemp, Tokens: defaultTokens}},
		}
		got := appendProfile(s)

		Expect(got.Items[1].Name).To(Equal("Профиль 6"))
	})

	It("refuses to delete the last profile", func() {
		s := profilesFromStored("", legacyBackend{})
		got, err := removeProfile(s, s.Active)

		Expect(err).To(MatchError(errLastProfile))
		Expect(got.Items).To(HaveLen(1))
		Expect(got.Active).To(Equal(s.Active))
	})

	It("selects the previous profile when the active one is deleted", func() {
		s := threeProfiles()
		got, err := removeProfile(s, "p2")

		Expect(err).NotTo(HaveOccurred())
		Expect(got.Active).To(Equal("p1"))
		Expect(got.Items).To(HaveLen(2))
		Expect(got.Items[0].ID).To(Equal("p1"))
		Expect(got.Items[1].ID).To(Equal("p3"))
	})

	It("selects the next profile when the first active one is deleted", func() {
		s := threeProfiles()
		s.Active = "p1"
		got, err := removeProfile(s, "p1")

		Expect(err).NotTo(HaveOccurred())
		Expect(got.Active).To(Equal("p2"))
	})

	It("keeps the active profile when another one is deleted", func() {
		s := threeProfiles()
		got, err := removeProfile(s, "p3")

		Expect(err).NotTo(HaveOccurred())
		Expect(got.Active).To(Equal("p2"))
		Expect(got.Items).To(HaveLen(2))
	})

	It("reports an unknown profile", func() {
		s := threeProfiles()
		_, err := removeProfile(s, "missing")
		Expect(err).To(MatchError(errUnknownProfile))

		_, err = selectProfile(s, "missing")
		Expect(err).To(MatchError(errUnknownProfile))
	})

	It("activates the chosen profile and leaves it unchanged when chosen again", func() {
		s := threeProfiles()
		got, err := selectProfile(s, "p3")
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Active).To(Equal("p3"))

		again, err := selectProfile(got, "p3")
		Expect(err).NotTo(HaveOccurred())
		Expect(again).To(Equal(got))
	})
})

func threeProfiles() profileSet {
	return profileSet{
		Active: "p2",
		Items: []backendProfile{
			{ID: "p1", Name: "A", URL: "http://a", Effort: effortLow, Temp: "0.1", Tokens: "1"},
			{ID: "p2", Name: "B", URL: "http://b", Effort: effortHigh, Temp: "0.3", Tokens: "2"},
			{ID: "p3", Name: "C", URL: "http://c", Effort: effortMedium, Temp: "0.2", Tokens: "3"},
		},
	}
}
