package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tomnagengast/compactor/internal/hookio"
	"github.com/tomnagengast/compactor/internal/transcript"
)

const manifestVersion = 1

type Manager struct{}

type Result struct {
	Manifest Manifest
}

type Manifest struct {
	Version            int        `json:"version"`
	Agent              string     `json:"agent"`
	SessionID          string     `json:"session_id"`
	CWD                string     `json:"cwd"`
	SessionDir         string     `json:"session_dir"`
	TranscriptPath     string     `json:"transcript_path,omitempty"`
	CreatedAt          string     `json:"created_at"`
	UpdatedAt          string     `json:"updated_at"`
	Events             []EventLog `json:"events"`
	Documents          []Document `json:"documents"`
	PendingContextPath string     `json:"pending_context_path"`
	Privacy            Privacy    `json:"privacy"`
}

type EventLog struct {
	EventName  string `json:"event_name"`
	Trigger    string `json:"trigger,omitempty"`
	TurnID     string `json:"turn_id,omitempty"`
	Source     string `json:"source,omitempty"`
	ReceivedAt string `json:"received_at"`
}

type Document struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	Summary       string `json:"summary"`
	RetrievalHint string `json:"retrieval_hint"`
}

type Privacy struct {
	RawTranscriptStored      bool   `json:"raw_transcript_stored"`
	BoundedTranscriptExtract bool   `json:"bounded_transcript_extract"`
	Notes                    string `json:"notes"`
}

func NewManager() Manager {
	return Manager{}
}

func (m Manager) PreCompact(event hookio.Event) (Result, error) {
	manifest, err := m.loadOrCreate(event)
	if err != nil {
		return Result{}, err
	}

	manifest.appendEvent(event)
	manifest.ensureDocuments(false)
	snapshotEvent := eventWithManifestTranscript(event, manifest)
	snapshot := transcriptSnapshot(snapshotEvent)
	manifest.Privacy.BoundedTranscriptExtract = len(snapshot.Entries) > 0
	if err := writeBaseDocuments(manifest, event, "precompact", snapshot); err != nil {
		return Result{}, err
	}
	if err := writePendingContext(manifest); err != nil {
		return Result{}, err
	}
	if err := writeManifest(manifest); err != nil {
		return Result{}, err
	}
	return Result{Manifest: manifest}, nil
}

func (m Manager) PostCompact(event hookio.Event) (Result, error) {
	manifest, err := m.loadOrCreate(event)
	if err != nil {
		return Result{}, err
	}

	manifest.appendEvent(event)
	manifest.ensureDocuments(event.CompactSummary != "")
	snapshotEvent := eventWithManifestTranscript(event, manifest)
	snapshot := transcriptSnapshot(snapshotEvent)
	manifest.Privacy.BoundedTranscriptExtract = len(snapshot.Entries) > 0
	if event.CompactSummary != "" {
		if err := os.MkdirAll(filepath.Join(manifest.SessionDir, "summaries"), 0o755); err != nil {
			return Result{}, err
		}
		if err := os.WriteFile(filepath.Join(manifest.SessionDir, "summaries", "native.md"), []byte(nativeSummaryMarkdown(event)), 0o600); err != nil {
			return Result{}, err
		}
	}
	if err := writeBaseDocuments(manifest, event, "postcompact", snapshot); err != nil {
		return Result{}, err
	}
	if err := writePendingContext(manifest); err != nil {
		return Result{}, err
	}
	if err := writeManifest(manifest); err != nil {
		return Result{}, err
	}
	return Result{Manifest: manifest}, nil
}

func (m Manager) PendingContext(event hookio.Event) (string, error) {
	manifest, err := m.loadOrCreate(event)
	if err != nil {
		return "", err
	}
	path := filepath.Join(manifest.SessionDir, "pending-context.md")
	content, err := os.ReadFile(path)
	if err == nil {
		return string(content), nil
	}
	manifest.ensureDocuments(false)
	if err := writePendingContext(manifest); err != nil {
		return "", err
	}
	content, err = os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (m Manager) loadOrCreate(event hookio.Event) (Manifest, error) {
	sessionDir := SessionDir(event)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return Manifest{}, err
	}

	path := filepath.Join(sessionDir, "manifest.json")
	if data, err := os.ReadFile(path); err == nil {
		var manifest Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return Manifest{}, fmt.Errorf("decode manifest: %w", err)
		}
		manifest.CWD = event.CWD
		if event.TranscriptPath != "" {
			manifest.TranscriptPath = event.TranscriptPath
		}
		manifest.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return manifest, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	return Manifest{
		Version:            manifestVersion,
		Agent:              string(event.Agent),
		SessionID:          hookio.SanitizePathComponent(event.SessionID),
		CWD:                event.CWD,
		SessionDir:         sessionDir,
		TranscriptPath:     event.TranscriptPath,
		CreatedAt:          now,
		UpdatedAt:          now,
		PendingContextPath: filepath.Join(sessionDir, "pending-context.md"),
		Privacy: Privacy{
			RawTranscriptStored:      false,
			BoundedTranscriptExtract: false,
			Notes:                    "Raw transcript content is not stored by default; generated docs may include bounded extracts.",
		},
	}, nil
}

func SessionDir(event hookio.Event) string {
	sessionID := hookio.SanitizePathComponent(event.SessionID)
	if sessionID == "" {
		sessionID = "unknown-session"
	}
	return filepath.Join(event.CWD, ".compactor", "sessions", string(event.Agent), sessionID)
}

func (manifest Manifest) RelativePath(path string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(manifest.SessionDir, path)
	}
	rel, err := filepath.Rel(manifest.CWD, path)
	if err != nil {
		return path
	}
	return rel
}

func (manifest *Manifest) appendEvent(event hookio.Event) {
	name := event.HookEventName
	if name == "" {
		name = inferredEventName(event)
	}
	manifest.Events = append(manifest.Events, EventLog{
		EventName:  name,
		Trigger:    event.Trigger,
		TurnID:     event.TurnID,
		Source:     event.Source,
		ReceivedAt: time.Now().UTC().Format(time.RFC3339),
	})
	manifest.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (manifest *Manifest) ensureDocuments(includeNativeSummary bool) {
	docs := []Document{
		{
			ID:            "index",
			Path:          filepath.Join(manifest.SessionDir, "index.md"),
			Kind:          "index",
			Summary:       "Map of compacted session documents and retrieval hints.",
			RetrievalHint: "you need to decide which compacted document to inspect",
		},
		{
			ID:            "timeline",
			Path:          filepath.Join(manifest.SessionDir, "timeline.md"),
			Kind:          "timeline",
			Summary:       "Bounded chronological extracts and metadata for the compacted session segment.",
			RetrievalHint: "you need prior sequence, source transcript metadata, or compacted message excerpts",
		},
		{
			ID:            "decisions",
			Path:          filepath.Join(manifest.SessionDir, "decisions.md"),
			Kind:          "decisions",
			Summary:       "Durable decisions and constraints extracted from compacted context.",
			RetrievalHint: "you need remembered decisions, constraints, or open questions",
		},
		{
			ID:            "tool-results",
			Path:          filepath.Join(manifest.SessionDir, "tool-results.md"),
			Kind:          "tool-results",
			Summary:       "Bounded references to tool calls and tool results found in compacted context.",
			RetrievalHint: "you need prior command, tool-call, or tool-result context",
		},
	}
	if includeNativeSummary {
		docs = append(docs, Document{
			ID:            "native-summary",
			Path:          filepath.Join(manifest.SessionDir, "summaries", "native.md"),
			Kind:          "summary",
			Summary:       "Native agent compaction summary captured after compaction.",
			RetrievalHint: "you need the exact native compact summary",
		})
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
	manifest.Documents = docs
}

func writeBaseDocuments(manifest Manifest, event hookio.Event, phase string, snapshot transcript.Snapshot) error {
	if err := os.WriteFile(filepath.Join(manifest.SessionDir, "index.md"), []byte(indexMarkdown(manifest)), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(manifest.SessionDir, "timeline.md"), []byte(timelineMarkdown(manifest, event, phase, snapshot)), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(manifest.SessionDir, "decisions.md"), []byte(decisionsMarkdown(manifest, snapshot)), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(manifest.SessionDir, "tool-results.md"), []byte(toolResultsMarkdown(snapshot)), 0o600); err != nil {
		return err
	}
	return nil
}

func writePendingContext(manifest Manifest) error {
	return os.WriteFile(filepath.Join(manifest.SessionDir, "pending-context.md"), []byte(pendingContextMarkdown(manifest)), 0o600)
}

func writeManifest(manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(manifest.SessionDir, "manifest.json"), data, 0o600)
}

func indexMarkdown(manifest Manifest) string {
	var b strings.Builder
	b.WriteString("# Compactor session index\n\n")
	b.WriteString("| Field | Value |\n| --- | --- |\n")
	b.WriteString("| Agent | `")
	b.WriteString(manifest.Agent)
	b.WriteString("` |\n")
	b.WriteString("| Session | `")
	b.WriteString(manifest.SessionID)
	b.WriteString("` |\n")
	if manifest.TranscriptPath != "" {
		b.WriteString("| Transcript | `")
		b.WriteString(manifest.TranscriptPath)
		b.WriteString("` |\n")
	}
	b.WriteString("\n## Documents\n\n")
	for _, doc := range manifest.Documents {
		b.WriteString("- `")
		b.WriteString(doc.ID)
		b.WriteString("`: [")
		b.WriteString(filepath.Base(doc.Path))
		b.WriteString("](")
		b.WriteString(manifest.RelativePath(doc.Path))
		b.WriteString(") - ")
		b.WriteString(doc.Summary)
		b.WriteString("\n")
	}
	b.WriteString("\n## Retrieval rule\n\nOpen these files only when compacted prior detail is needed for the current task.\n")
	return b.String()
}

func timelineMarkdown(manifest Manifest, event hookio.Event, phase string, snapshot transcript.Snapshot) string {
	var b strings.Builder
	b.WriteString("# Compacted timeline\n\n")
	b.WriteString("This file records metadata and bounded extracts for the compacted segment. Raw transcript content is not copied in full.\n\n")
	b.WriteString("## Latest hook\n\n")
	b.WriteString("- Phase: `")
	b.WriteString(phase)
	b.WriteString("`\n")
	if event.Trigger != "" {
		b.WriteString("- Trigger: `")
		b.WriteString(event.Trigger)
		b.WriteString("`\n")
	}
	if event.TurnID != "" {
		b.WriteString("- Turn: `")
		b.WriteString(event.TurnID)
		b.WriteString("`\n")
	}
	if manifest.TranscriptPath != "" {
		b.WriteString("- Transcript: `")
		b.WriteString(manifest.TranscriptPath)
		b.WriteString("`\n")
		if info, err := os.Stat(manifest.TranscriptPath); err == nil {
			b.WriteString("- Transcript bytes: `")
			b.WriteString(fmt.Sprint(info.Size()))
			b.WriteString("`\n")
		}
	}
	if snapshot.ParseError != "" {
		b.WriteString("- Transcript parse error: `")
		b.WriteString(snapshot.ParseError)
		b.WriteString("`\n")
	}
	b.WriteString("\n## Event log\n\n")
	for _, logged := range manifest.Events {
		b.WriteString("- `")
		b.WriteString(logged.EventName)
		b.WriteString("`")
		if logged.Trigger != "" {
			b.WriteString(" trigger=`")
			b.WriteString(logged.Trigger)
			b.WriteString("`")
		}
		if logged.TurnID != "" {
			b.WriteString(" turn=`")
			b.WriteString(logged.TurnID)
			b.WriteString("`")
		}
		b.WriteString("\n")
	}
	if len(snapshot.Boundaries) > 0 {
		b.WriteString("\n## Compaction boundaries\n\n")
		for _, finding := range snapshot.Boundaries {
			b.WriteString("- line ")
			b.WriteString(fmt.Sprint(finding.Line))
			b.WriteString(" ")
			b.WriteString(finding.Kind)
			b.WriteString(": ")
			b.WriteString(finding.Text)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n## Bounded transcript extracts\n\n")
	if len(snapshot.Entries) == 0 {
		b.WriteString("No transcript entries were extracted.\n")
		return b.String()
	}
	for _, entry := range snapshot.Entries {
		b.WriteString("- line ")
		b.WriteString(fmt.Sprint(entry.Line))
		if entry.Timestamp != "" {
			b.WriteString(" `")
			b.WriteString(entry.Timestamp)
			b.WriteString("`")
		}
		if entry.Role != "" {
			b.WriteString(" ")
			b.WriteString(entry.Role)
		}
		if entry.Kind != "" && entry.Kind != entry.Role {
			b.WriteString("/")
			b.WriteString(entry.Kind)
		}
		if entry.ID != "" {
			b.WriteString(" id=`")
			b.WriteString(entry.ID)
			b.WriteString("`")
		}
		if entry.ParentID != "" {
			b.WriteString(" parent=`")
			b.WriteString(entry.ParentID)
			b.WriteString("`")
		}
		if entry.ToolName != "" {
			b.WriteString(" tool=`")
			b.WriteString(entry.ToolName)
			b.WriteString("`")
		}
		b.WriteString(": ")
		b.WriteString(entry.Text)
		b.WriteString("\n")
	}
	return b.String()
}

func decisionsMarkdown(_ Manifest, snapshot transcript.Snapshot) string {
	var b strings.Builder
	b.WriteString("# Decisions and constraints\n\n")
	b.WriteString("These are bounded heuristic candidates extracted from compacted context. Verify against the source transcript before treating them as authoritative.\n\n")
	if len(snapshot.Decisions) == 0 {
		b.WriteString("No decision candidates were extracted.\n")
		return b.String()
	}
	for _, finding := range snapshot.Decisions {
		b.WriteString("- line ")
		b.WriteString(fmt.Sprint(finding.Line))
		b.WriteString(" ")
		b.WriteString(finding.Kind)
		b.WriteString(": ")
		b.WriteString(finding.Text)
		b.WriteString("\n")
	}
	return b.String()
}

func toolResultsMarkdown(snapshot transcript.Snapshot) string {
	var b strings.Builder
	b.WriteString("# Tool results\n\n")
	b.WriteString("These are bounded references to tool calls or tool results extracted from compacted context. Full raw tool output is not copied by default.\n\n")
	if len(snapshot.ToolResults) == 0 {
		b.WriteString("No tool result candidates were extracted.\n")
		return b.String()
	}
	for _, finding := range snapshot.ToolResults {
		b.WriteString("- line ")
		b.WriteString(fmt.Sprint(finding.Line))
		b.WriteString(" ")
		b.WriteString(finding.Kind)
		b.WriteString(": ")
		b.WriteString(finding.Text)
		b.WriteString("\n")
	}
	return b.String()
}

func nativeSummaryMarkdown(event hookio.Event) string {
	return "# Native compact summary\n\n" + strings.TrimSpace(event.CompactSummary) + "\n"
}

func pendingContextMarkdown(manifest Manifest) string {
	var b strings.Builder
	b.WriteString("Compactor preserved compacted history for this session.\n\n")
	b.WriteString("Index: ")
	b.WriteString(manifest.RelativePath(filepath.Join(manifest.SessionDir, "index.md")))
	b.WriteString("\n\n")
	b.WriteString("Available refs:\n")
	for _, doc := range manifest.Documents {
		b.WriteString("- ")
		b.WriteString(doc.ID)
		b.WriteString(": ")
		b.WriteString(manifest.RelativePath(doc.Path))
		b.WriteString(" - ")
		b.WriteString(doc.Summary)
		if doc.RetrievalHint != "" {
			b.WriteString(" Open when ")
			b.WriteString(doc.RetrievalHint)
			b.WriteString(".")
		}
		b.WriteString("\n")
	}
	b.WriteString("\nOpen the index only when compacted prior detail is needed.\n")
	return b.String()
}

func inferredEventName(event hookio.Event) string {
	if event.Trigger != "" {
		return "Compact"
	}
	if event.Source != "" {
		return "SessionStart"
	}
	return "Unknown"
}

func transcriptSnapshot(event hookio.Event) transcript.Snapshot {
	if event.TranscriptPath == "" {
		return transcript.Snapshot{}
	}
	snapshot, err := transcript.Read(event.TranscriptPath, transcript.Options{Agent: string(event.Agent)})
	if err != nil {
		return transcript.Snapshot{
			SourcePath: event.TranscriptPath,
			ParseError: err.Error(),
		}
	}
	return snapshot
}

func eventWithManifestTranscript(event hookio.Event, manifest Manifest) hookio.Event {
	if event.TranscriptPath == "" {
		event.TranscriptPath = manifest.TranscriptPath
	}
	return event
}
