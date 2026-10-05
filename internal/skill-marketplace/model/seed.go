package model

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// initialSkillsSeedKey names this batch in skill_seed_runs. A future batch
// gets a new key; never reuse or rename this one, or production re-seeds.
const initialSkillsSeedKey = "2026-10-video-skills"

type initialSkill struct {
	Slug        string
	Name        string
	Description string
	Tags        []string
	SourceURL   string
}

// initialSkills are open-source, OSI-licensed video-production skills listed
// as free reference skills (link only, nothing redistributed). Licenses and
// paths verified against GitHub on 2026-10-05.
var initialSkills = []initialSkill{
	{
		Slug:        "video-use",
		Name:        "video-use - Conversational Video Editing",
		Description: "Edit a video by talking to Claude: transcribe, cut on word boundaries, color grade, add overlay animations and burn in subtitles. Open source by Browser Use (MIT). Needs ffmpeg; transcription uses an ElevenLabs Scribe API key.",
		Tags:        []string{"video"},
		SourceURL:   "https://github.com/browser-use/video-use",
	},
	{
		Slug:        "clipify",
		Name:        "clipify - Long Video to Social Clips",
		Description: "Finds the strongest moments in a long video, cuts them into standalone clips, reframes 16:9 to 9:16 and burns in word-by-word captions. Runs locally with ffmpeg and Whisper, no paid API. Open source (MIT).",
		Tags:        []string{"video"},
		SourceURL:   "https://github.com/louisedesadeleer/clipify",
	},
	{
		Slug:        "openai-transcribe",
		Name:        "transcribe - Audio and Video Transcription",
		Description: "Transcribes speech from audio or video files to text, with optional speaker labels - a starting point for subtitles and show notes. Official OpenAI skill (Apache-2.0); needs an OpenAI API key.",
		Tags:        []string{"video"},
		SourceURL:   "https://github.com/openai/skills/tree/main/skills/.curated/transcribe",
	},
	{
		Slug:        "openai-speech",
		Name:        "speech - Text-to-Speech Voiceover",
		Description: "Turns a script into spoken narration or voiceover with built-in voices, one clip or a batch. Official OpenAI skill (Apache-2.0); needs an OpenAI API key.",
		Tags:        []string{"video"},
		SourceURL:   "https://github.com/openai/skills/tree/main/skills/.curated/speech",
	},
	{
		Slug:        "ffmpeg-video-processing",
		Name:        "ffmpeg - Video and Audio Processing",
		Description: "Recipes for converting, resizing, compressing and trimming video, extracting audio and preparing assets for editing. From the Claude Code Video Toolkit by Digital Samba (MIT); needs only a local ffmpeg.",
		Tags:        []string{"video"},
		SourceURL:   "https://github.com/digitalsamba/claude-code-video-toolkit/tree/main/.claude/skills/ffmpeg",
	},
	{
		Slug:        "youtube-script-writer",
		Name:        "youtube-script-writer - Video Script Writing",
		Description: "Writes high-retention scripts for explainers, tutorials, reviews and talking-head videos: packaging, cold open, value stack and retention beats. Open source (MIT); no extra tools or API keys.",
		Tags:        []string{"video", "writing"},
		SourceURL:   "https://github.com/mohitagw15856/pm-claude-skills/tree/main/skills/youtube-script-writer",
	},
}

// seedInitialSkills writes initialSkills once per database, recorded in
// skill_seed_runs so that an Admin later deleting or editing a seeded skill
// is never undone by a restart. An existing slug is left alone. With no root
// admin yet (fresh install) it skips without recording, so the next start
// retries.
func seedInitialSkills(db *gorm.DB) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS skill_seed_runs (
		seed_key   VARCHAR(100) PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL
	)`).Error; err != nil {
		return err
	}

	var done int64
	if err := db.Raw(`SELECT COUNT(*) FROM skill_seed_runs WHERE seed_key = ?`, initialSkillsSeedKey).Scan(&done).Error; err != nil {
		return err
	}
	if done > 0 {
		return nil
	}

	rootQuery := `SELECT id FROM users WHERE role >= ?`
	if db.Migrator().HasColumn("users", "deleted_at") {
		rootQuery += ` AND deleted_at IS NULL`
	}
	var rootIDs []int64
	if err := db.Raw(rootQuery+` ORDER BY id LIMIT 1`, common.RoleRootUser).Scan(&rootIDs).Error; err != nil {
		return err
	}
	if len(rootIDs) == 0 {
		common.SysLog("skill marketplace: no root admin yet, initial skills will be seeded on a later start")
		return nil
	}
	adminID := rootIDs[0]

	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		inserted := 0
		for _, s := range initialSkills {
			var ids []int64
			if err := tx.Raw(
				`INSERT INTO skills (slug, name, description, tags, status, monetization_type, price_usd,
				                     listing_type, source_url, created_by, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, 'free', 0, ?, ?, ?, ?, ?)
				 ON CONFLICT (slug) DO NOTHING
				 RETURNING id`,
				s.Slug, s.Name, s.Description, pq.StringArray(s.Tags), SkillStatusPublished,
				SkillListingTypeReference, s.SourceURL, adminID, now, now,
			).Scan(&ids).Error; err != nil {
				return fmt.Errorf("seed skill %q: %w", s.Slug, err)
			}
			if len(ids) == 0 {
				continue
			}
			inserted++
			if err := WriteLog(tx, adminID, &ids[0], LogActionCreate,
				map[string]string{"source": "seed", "seed_key": initialSkillsSeedKey}); err != nil {
				return fmt.Errorf("seed skill %q audit log: %w", s.Slug, err)
			}
		}
		if err := tx.Exec(`INSERT INTO skill_seed_runs (seed_key, applied_at) VALUES (?, ?)`,
			initialSkillsSeedKey, now).Error; err != nil {
			return err
		}
		common.SysLog(fmt.Sprintf("skill marketplace: seeded %d initial skills (%s)", inserted, initialSkillsSeedKey))
		return nil
	})
}
