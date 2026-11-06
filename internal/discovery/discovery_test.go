package discovery_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	_ "modernc.org/sqlite"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/ai/transport"
	"sea-noise/internal/discovery"
	"sea-noise/internal/retrieval"
)

type discoveryFixture struct {
	db       *sql.DB
	root     string
	overview string
	field    string
	reading  string
	linked   string
}

func newDiscoveryFixture(t *testing.T, extra ...map[string]string) *discoveryFixture {
	t.Helper()
	root := t.TempDir()
	// Exercise indexed project context; reserved wing paths require enrollment.
	files := map[string]string{
		"project-context/acme/overview.md": "# Acme\n\nThe lantern project studies humane field research.\n",
		"notes/field.md":                   "# Field notes\n\nLantern observations connect careful walking with patient listening. [[linked]]\n",
		"notes/reading.md":                 "# Reading\n\nLantern research describes patient listening as a field method.\n",
		"notes/linked.md":                  "# Linked note\n\nA graph-only companion describes patient listening as a field method.\n",
	}
	for _, additions := range extra {
		for name, body := range additions {
			files[name] = body
		}
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := retrieval.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := retrieval.IndexVault(context.Background(), db, root, nil); err != nil {
		t.Fatalf("index: %v", err)
	}

	return &discoveryFixture{
		db:       db,
		root:     root,
		overview: "project-context/acme/overview.md",
		field:    "notes/field.md",
		reading:  "notes/reading.md",
		linked:   "notes/linked.md",
	}
}

type transportFunc func(context.Context, inferencerouter.InferenceDecision, string) (string, error)

func (f transportFunc) Run(ctx context.Context, decision inferencerouter.InferenceDecision, prompt string) (string, error) {
	return f(ctx, decision, prompt)
}

func newExecutor(tx transport.Transport) *skill.Executor {
	return skill.NewExecutor(
		map[string]*skill.Skill{
			"discover-expand-query": {
				Name: "discover-expand-query",
				Kind: "inference",
				Body: "{{.query}} {{.project_context}}",
			},
			"discover-explain-association": {
				Name: "discover-explain-association",
				Kind: "inference",
				Body: "{{.query}} {{.project_context}} {{.sources}}",
			},
		},
		inferencerouter.NewLocalRouter(),
		tx,
	)
}

func chunkFor(t *testing.T, db *sql.DB, noteID, query string) (int64, string) {
	t.Helper()
	var id int64
	var body string
	if err := db.QueryRow(`
		SELECT c.id, c.body
		FROM chunks c
		JOIN chunks_fts ON chunks_fts.rowid = c.id
		WHERE c.note_id = ? AND chunks_fts MATCH ?
		ORDER BY c.ordinal, c.id
		LIMIT 1`, noteID, query).Scan(&id, &body); err != nil {
		t.Fatalf("matching chunk %s/%s: %v", noteID, query, err)
	}
	return id, body
}

func associationResponse(t *testing.T, candidateID, candidatePassage, otherID, otherPassage string) string {
	t.Helper()
	payload := map[string]any{
		"connections": []map[string]any{{
			"candidate_id": candidateID,
			"explanation":  "Both notes connect patient listening with field observation.",
			"evidence": []map[string]string{
				{"source_id": candidateID, "passage": candidatePassage},
				{"source_id": otherID, "passage": otherPassage},
			},
		}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return string(raw)
}

func discoverRequest(f *discoveryFixture, mock *transport.MockTransport) (discovery.Result, error) {
	return discovery.Discover(context.Background(), discovery.Config{
		DB:          f.db,
		Executor:    newExecutor(mock),
		ReadOnlyCLI: true,
	}, discovery.Request{
		Query:             "lantern",
		ProjectOverviewID: f.overview,
		Limit:             5,
	})
}

func TestDiscoverGroundsConnectionsInIndexedChunksAndDoesNotWriteSources(t *testing.T) {
	f := newDiscoveryFixture(t)
	fieldID, fieldBody := chunkFor(t, f.db, f.field, "lantern")
	readingID, readingBody := chunkFor(t, f.db, f.reading, "lantern")
	overviewPath := filepath.Join(f.root, filepath.FromSlash(f.overview))
	before, err := os.ReadFile(overviewPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeHash := sha256.Sum256(before)

	mock := &transport.MockTransport{}
	mock.EnqueueResponses(associationResponse(
		t,
		discovery.SourceID(f.field, fieldID), strings.TrimSpace(fieldBody),
		discovery.SourceID(f.reading, readingID), strings.TrimSpace(readingBody),
	))
	result, err := discoverRequest(f, mock)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(result.Sources) < 3 {
		t.Fatalf("sources = %d, want overview plus candidates", len(result.Sources))
	}
	foundGraph := false
	for _, source := range result.Sources {
		if source.NoteID == f.linked && !source.Context {
			foundGraph = true
			break
		}
	}
	if !foundGraph {
		t.Fatalf("sources = %+v, want one-hop graph source %s", result.Sources, f.linked)
	}
	var explained *discovery.Candidate
	for i := range result.Candidates {
		if result.Candidates[i].Explanation != "" {
			explained = &result.Candidates[i]
			break
		}
	}
	if explained == nil {
		t.Fatalf("candidates = %+v, want grounded explanation", result.Candidates)
	}
	if len(explained.Evidence) != 2 {
		t.Fatalf("evidence = %+v, want two citations", explained.Evidence)
	}
	after, err := os.ReadFile(overviewPath)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(after) != beforeHash {
		t.Fatal("discovery changed the indexed source file")
	}
}

func TestDiscoverLimitOneRetainsGroundedCrossNoteEvidence(t *testing.T) {
	f := newDiscoveryFixture(t, map[string]string{
		"notes/field.md":   "# Field notes\n\nCobalt observations connect careful walking with patient listening.\n",
		"notes/reading.md": "# Reading\n\nCobalt research describes patient listening as a field method.\n",
	})
	fieldID, fieldBody := chunkFor(t, f.db, f.field, "cobalt")
	readingID, readingBody := chunkFor(t, f.db, f.reading, "cobalt")
	fieldSource := discovery.SourceID(f.field, fieldID)
	readingSource := discovery.SourceID(f.reading, readingID)
	mock := &transport.MockTransport{}
	mock.EnqueueResponses(associationResponse(
		t,
		fieldSource, strings.TrimSpace(fieldBody),
		readingSource, strings.TrimSpace(readingBody),
	))

	result, err := discovery.Discover(context.Background(), discovery.Config{
		DB:          f.db,
		Executor:    newExecutor(mock),
		ReadOnlyCLI: true,
	}, discovery.Request{Query: "cobalt", Limit: 1})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want one grounded cross-note connection", result.Candidates)
	}
	candidate := result.Candidates[0]
	if candidate.SourceID != fieldSource || candidate.NoteID != f.field || candidate.ChunkID != fieldID || candidate.Explanation == "" {
		t.Fatalf("candidate = %+v, want explained field-note candidate %s", candidate, fieldSource)
	}
	if len(candidate.Evidence) != 2 {
		t.Fatalf("evidence = %+v, want two distinct grounded notes", candidate.Evidence)
	}
	want := map[string]discovery.Evidence{
		fieldSource:   {SourceID: fieldSource, NoteID: f.field, ChunkID: fieldID, Passage: strings.TrimSpace(fieldBody)},
		readingSource: {SourceID: readingSource, NoteID: f.reading, ChunkID: readingID, Passage: strings.TrimSpace(readingBody)},
	}
	for _, evidence := range candidate.Evidence {
		expected, ok := want[evidence.SourceID]
		if !ok || evidence != expected {
			t.Fatalf("evidence = %+v, want exact indexed citation %+v", evidence, expected)
		}
		delete(want, evidence.SourceID)
	}
	if len(want) != 0 {
		t.Fatalf("missing cross-note evidence: %+v", want)
	}
}

func TestDiscoverRebuildDuringExpansionUsesFinalIndexSnapshot(t *testing.T) {
	f := newDiscoveryFixture(t)
	oldOverviewID, _ := chunkFor(t, f.db, f.overview, "lantern")
	oldOverviewHeadingID, _ := chunkFor(t, f.db, f.overview, "Acme")
	oldFieldID, _ := chunkFor(t, f.db, f.field, "lantern")
	oldReadingID, _ := chunkFor(t, f.db, f.reading, "lantern")
	var (
		calls                int
		newOverviewID        int64
		newOverviewBody      string
		newOverviewHeadingID int64
		newOverviewHeading   string
		newFieldID           int64
		newFieldBody         string
		newReadingID         int64
		newReadingBody       string
	)
	executor := newExecutor(transportFunc(func(ctx context.Context, _ inferencerouter.InferenceDecision, _ string) (string, error) {
		calls++
		switch calls {
		case 1:
			files := map[string]string{
				"project-context/acme/overview.md": "# Acme\n\nRenewed context for the rebuilt index.\n",
				"notes/field.md":                   "# Field notes\n\nAtlas observations connect careful walking with patient listening.\n",
				"notes/reading.md":                 "# Reading\n\nAtlas research describes patient listening as a field method.\n",
				"notes/linked.md":                  "# Linked note\n\nAn unrelated graph companion.\n",
			}
			for name, body := range files {
				if err := os.WriteFile(filepath.Join(f.root, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
					return "", err
				}
			}
			if _, err := retrieval.IndexVault(ctx, f.db, f.root, nil); err != nil {
				return "", err
			}
			newOverviewID, newOverviewBody = chunkFor(t, f.db, f.overview, "renewed")
			newOverviewHeadingID, newOverviewHeading = chunkFor(t, f.db, f.overview, "Acme")
			newFieldID, newFieldBody = chunkFor(t, f.db, f.field, "atlas")
			newReadingID, newReadingBody = chunkFor(t, f.db, f.reading, "atlas")
			return `{"queries":["atlas"]}`, nil
		case 2:
			return associationResponse(
				t,
				discovery.SourceID(f.field, newFieldID), strings.TrimSpace(newFieldBody),
				discovery.SourceID(f.reading, newReadingID), strings.TrimSpace(newReadingBody),
			), nil
		default:
			return "", errors.New("unexpected discovery transport call")
		}
	}))

	result, err := discovery.Discover(context.Background(), discovery.Config{
		DB:          f.db,
		Executor:    executor,
		ReadOnlyCLI: true,
	}, discovery.Request{
		Query:             "lantern",
		ProjectOverviewID: f.overview,
		ExpandQueries:     true,
		Limit:             1,
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if calls != 2 {
		t.Fatalf("transport calls = %d, want expansion and association calls", calls)
	}
	if len(result.Queries) != 2 || result.Queries[1] != "atlas" {
		t.Fatalf("queries = %v, want rebuilt-index expansion", result.Queries)
	}
	oldIDs := map[string]struct{}{
		discovery.SourceID(f.overview, oldOverviewID):        {},
		discovery.SourceID(f.overview, oldOverviewHeadingID): {},
		discovery.SourceID(f.field, oldFieldID):              {},
		discovery.SourceID(f.reading, oldReadingID):          {},
	}
	newOverviewSources := map[string]string{
		discovery.SourceID(f.overview, newOverviewID):        strings.TrimSpace(newOverviewBody),
		discovery.SourceID(f.overview, newOverviewHeadingID): strings.TrimSpace(newOverviewHeading),
	}
	if newOverviewID == oldOverviewID || newOverviewHeadingID == oldOverviewHeadingID || newFieldID == oldFieldID || newReadingID == oldReadingID {
		t.Fatalf("rebuild reused old chunk identities: old overview=%d heading=%d field=%d reading=%d, new overview=%d heading=%d field=%d reading=%d", oldOverviewID, oldOverviewHeadingID, oldFieldID, oldReadingID, newOverviewID, newOverviewHeadingID, newFieldID, newReadingID)
	}
	for _, source := range result.Sources {
		if _, old := oldIDs[source.SourceID]; old {
			t.Fatalf("public sources contain old snapshot source: %+v", source)
		}
		if source.NoteID == f.overview {
			expected, ok := newOverviewSources[source.SourceID]
			if !ok || !source.Context || strings.TrimSpace(source.Passage) != expected {
				t.Fatalf("overview source = %+v, want renewed final context", source)
			}
			delete(newOverviewSources, source.SourceID)
		}
	}
	if len(newOverviewSources) != 0 {
		t.Fatalf("sources = %+v, missing renewed overview context", result.Sources)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want one rebuilt candidate", result.Candidates)
	}
	candidate := result.Candidates[0]
	newFieldSource := discovery.SourceID(f.field, newFieldID)
	newReadingSource := discovery.SourceID(f.reading, newReadingID)
	if candidate.SourceID != newFieldSource || candidate.NoteID != f.field || candidate.ChunkID != newFieldID {
		t.Fatalf("candidate = %+v, want rebuilt field identity %s", candidate, newFieldSource)
	}
	wantEvidence := map[string]discovery.Evidence{
		newFieldSource:   {SourceID: newFieldSource, NoteID: f.field, ChunkID: newFieldID, Passage: strings.TrimSpace(newFieldBody)},
		newReadingSource: {SourceID: newReadingSource, NoteID: f.reading, ChunkID: newReadingID, Passage: strings.TrimSpace(newReadingBody)},
	}
	if len(candidate.Evidence) != len(wantEvidence) {
		t.Fatalf("candidate evidence = %+v, want rebuilt evidence from both notes", candidate.Evidence)
	}
	for _, evidence := range candidate.Evidence {
		expected, ok := wantEvidence[evidence.SourceID]
		if !ok || evidence != expected {
			t.Fatalf("candidate evidence = %+v, want exact rebuilt citation %+v", evidence, expected)
		}
		delete(wantEvidence, evidence.SourceID)
	}
	if len(wantEvidence) != 0 {
		t.Fatalf("missing rebuilt evidence: %+v", wantEvidence)
	}
}

func TestDiscoverRejectsUnknownEvidence(t *testing.T) {
	f := newDiscoveryFixture(t)
	fieldID, fieldBody := chunkFor(t, f.db, f.field, "lantern")
	mock := &transport.MockTransport{}
	mock.EnqueueResponses(associationResponse(
		t,
		discovery.SourceID(f.field, fieldID), strings.TrimSpace(fieldBody),
		"not-indexed#chunk:999", "fabricated passage",
	))
	_, err := discoverRequest(f, mock)
	if !errors.Is(err, discovery.ErrUnknownEvidence) {
		t.Fatalf("err = %v, want ErrUnknownEvidence", err)
	}
}

func TestDiscoverRejectsMalformedAssociationOutput(t *testing.T) {
	f := newDiscoveryFixture(t)
	mock := &transport.MockTransport{}
	mock.EnqueueResponses("not-json")
	_, err := discoverRequest(f, mock)
	if !errors.Is(err, discovery.ErrMalformedOutput) {
		t.Fatalf("err = %v, want ErrMalformedOutput", err)
	}
}

func TestDiscoverRequiresReadOnlyCLIAttestation(t *testing.T) {
	f := newDiscoveryFixture(t)
	mock := &transport.MockTransport{}
	_, err := discovery.Discover(context.Background(), discovery.Config{
		DB:          f.db,
		Executor:    newExecutor(mock),
		ReadOnlyCLI: false,
	}, discovery.Request{Query: "lantern"})
	if !errors.Is(err, discovery.ErrReadOnlyRequired) {
		t.Fatalf("err = %v, want ErrReadOnlyRequired", err)
	}
	if len(mock.Calls) != 0 {
		t.Fatalf("transport calls = %d, want 0", len(mock.Calls))
	}
}
func TestDiscoverRejectsKnownEvidenceWithFabricatedPassage(t *testing.T) {
	f := newDiscoveryFixture(t)
	fieldID, _ := chunkFor(t, f.db, f.field, "lantern")
	readingID, readingBody := chunkFor(t, f.db, f.reading, "lantern")
	mock := &transport.MockTransport{}
	mock.EnqueueResponses(associationResponse(
		t,
		discovery.SourceID(f.field, fieldID), "fabricated passage",
		discovery.SourceID(f.reading, readingID), strings.TrimSpace(readingBody),
	))
	_, err := discoverRequest(f, mock)
	if !errors.Is(err, discovery.ErrUnknownEvidence) {
		t.Fatalf("err = %v, want ErrUnknownEvidence", err)
	}
}

func TestDiscoverRejectsOneNoteEvidence(t *testing.T) {
	f := newDiscoveryFixture(t)
	fieldID, fieldBody := chunkFor(t, f.db, f.field, "lantern")
	mock := &transport.MockTransport{}
	mock.EnqueueResponses(associationResponse(
		t,
		discovery.SourceID(f.field, fieldID), strings.TrimSpace(fieldBody),
		discovery.SourceID(f.field, fieldID), strings.TrimSpace(fieldBody),
	))
	_, err := discoverRequest(f, mock)
	if !errors.Is(err, discovery.ErrMissingEvidence) {
		t.Fatalf("err = %v, want ErrMissingEvidence", err)
	}
}
func TestDiscoverRejectsAssociationOutputOverLimit(t *testing.T) {
	f := newDiscoveryFixture(t)
	fieldID, fieldBody := chunkFor(t, f.db, f.field, "lantern")
	readingID, readingBody := chunkFor(t, f.db, f.reading, "lantern")
	fieldSource := discovery.SourceID(f.field, fieldID)
	readingSource := discovery.SourceID(f.reading, readingID)
	raw, err := json.Marshal(map[string]any{
		"connections": []map[string]any{
			{
				"candidate_id": fieldSource,
				"explanation":  "Both notes connect patient listening with field observation.",
				"evidence": []map[string]string{
					{"source_id": fieldSource, "passage": strings.TrimSpace(fieldBody)},
					{"source_id": readingSource, "passage": strings.TrimSpace(readingBody)},
				},
			},
			{
				"candidate_id": readingSource,
				"explanation":  "Both notes connect patient listening with field observation.",
				"evidence": []map[string]string{
					{"source_id": readingSource, "passage": strings.TrimSpace(readingBody)},
					{"source_id": fieldSource, "passage": strings.TrimSpace(fieldBody)},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mock := &transport.MockTransport{}
	mock.EnqueueResponses(string(raw))
	_, err = discovery.Discover(context.Background(), discovery.Config{
		DB:          f.db,
		Executor:    newExecutor(mock),
		ReadOnlyCLI: true,
	}, discovery.Request{Query: "lantern", Limit: 1})
	if !errors.Is(err, discovery.ErrMalformedOutput) {
		t.Fatalf("err = %v, want ErrMalformedOutput", err)
	}
}

func TestDiscoverUsesExpansionAndGraphRecallWithoutProjectFiltering(t *testing.T) {
	f := newDiscoveryFixture(t)
	fieldID, fieldBody := chunkFor(t, f.db, f.field, "lantern")
	linkedID, linkedBody := chunkFor(t, f.db, f.linked, "patient")
	mock := &transport.MockTransport{}
	mock.EnqueueResponses(
		`{"queries":["careful walking"]}`,
		associationResponse(
			t,
			discovery.SourceID(f.field, fieldID), strings.TrimSpace(fieldBody),
			discovery.SourceID(f.linked, linkedID), strings.TrimSpace(linkedBody),
		),
	)
	result, err := discovery.Discover(context.Background(), discovery.Config{
		DB:          f.db,
		Executor:    newExecutor(mock),
		ReadOnlyCLI: true,
	}, discovery.Request{
		Query:             "lantern",
		ProjectOverviewID: f.overview,
		ExpandQueries:     true,
		Limit:             2,
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(result.Queries) != 2 || result.Queries[1] != "careful walking" {
		t.Fatalf("queries = %v, want original plus expansion", result.Queries)
	}
	if len(result.Candidates) > 2 {
		t.Fatalf("candidates = %d, want limit <= 2", len(result.Candidates))
	}
	foundContext := false
	foundGraph := false
	for _, source := range result.Sources {
		if source.NoteID == f.overview && source.Context {
			foundContext = true
		}
		if source.NoteID == f.linked && !source.Context {
			foundGraph = true
		}
	}
	if !foundContext {
		t.Fatalf("sources = %+v, want project overview context", result.Sources)
	}
	if !foundGraph {
		t.Fatalf("sources = %+v, want graph-linked source", result.Sources)
	}
	linkedSourceID := discovery.SourceID(f.linked, linkedID)
	linkedPassage := strings.TrimSpace(linkedBody)
	foundLinkedEvidence := false
	for _, candidate := range result.Candidates {
		for _, evidence := range candidate.Evidence {
			if evidence.SourceID == linkedSourceID && evidence.Passage == linkedPassage {
				foundLinkedEvidence = true
				break
			}
		}
	}
	if !foundLinkedEvidence {
		t.Fatalf("candidates = %+v, want exact linked evidence %s", result.Candidates, linkedSourceID)
	}
}
func TestDiscoverQuotesNaturalLanguageQueriesForFTS(t *testing.T) {
	f := newDiscoveryFixture(t)
	fieldID, fieldBody := chunkFor(t, f.db, f.field, "lantern")
	linkedID, linkedBody := chunkFor(t, f.db, f.linked, "patient")
	query := `lantern? "field"`
	mock := &transport.MockTransport{}
	mock.EnqueueResponses(
		`{"queries":["patient?"]}`,
		associationResponse(
			t,
			discovery.SourceID(f.field, fieldID), strings.TrimSpace(fieldBody),
			discovery.SourceID(f.linked, linkedID), strings.TrimSpace(linkedBody),
		),
	)
	result, err := discovery.Discover(context.Background(), discovery.Config{
		DB:          f.db,
		Executor:    newExecutor(mock),
		ReadOnlyCLI: true,
	}, discovery.Request{
		Query:         query,
		ExpandQueries: true,
		Limit:         2,
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if result.Query != query || len(result.Queries) != 2 || result.Queries[0] != query || result.Queries[1] != "patient?" {
		t.Fatalf("queries = %v, want original and punctuated expansion preserved", result.Queries)
	}
	if len(mock.Calls) != 2 {
		t.Fatalf("transport calls = %d, want expansion plus association", len(mock.Calls))
	}
	linkedSourceID := discovery.SourceID(f.linked, linkedID)
	linkedPassage := strings.TrimSpace(linkedBody)
	foundLinkedEvidence := false
	for _, candidate := range result.Candidates {
		for _, evidence := range candidate.Evidence {
			if evidence.SourceID == linkedSourceID && evidence.Passage == linkedPassage {
				foundLinkedEvidence = true
				break
			}
		}
	}
	if !foundLinkedEvidence {
		t.Fatalf("candidates = %+v, want validated linked evidence", result.Candidates)
	}
}

func TestDiscoverPublishesOnlyAdmittedSourceManifest(t *testing.T) {
	files := map[string]string{}
	overviewParts := make([]string, 0, 4)
	for i := 1; i <= 4; i++ {
		marker := fmt.Sprintf("overview-%02d", i)
		overviewParts = append(overviewParts, marker+" "+strings.Repeat("overview-filler ", 220))
	}
	files["project-context/acme/overview.md"] = strings.Join(overviewParts, "\n\n")
	for i := 1; i <= 24; i++ {
		marker := fmt.Sprintf("candidate-%02d", i)
		files["notes/"+marker+".md"] = marker + " " + strings.Repeat("candidate-filler ", 220)
	}
	f := newDiscoveryFixture(t, files)

	firstMock := &transport.MockTransport{}
	firstMock.EnqueueResponses(`{"connections":[]}`)
	first, err := discovery.Discover(context.Background(), discovery.Config{
		DB:          f.db,
		Executor:    newExecutor(firstMock),
		ReadOnlyCLI: true,
	}, discovery.Request{
		Query:             "candidate",
		ProjectOverviewID: f.overview,
		Limit:             1,
	})
	if err != nil {
		t.Fatalf("first Discover: %v", err)
	}
	if len(first.Candidates) != 0 {
		t.Fatalf("first candidates = %+v, want empty association result", first.Candidates)
	}

	var candidate discovery.Source
	var contextSource discovery.Source
	haveCandidate := false
	haveContext := false
	contextCount := 0
	for _, source := range first.Sources {
		if source.Context {
			contextCount++
			if !haveContext {
				contextSource = source
				haveContext = true
			}
			continue
		}
		candidate = source
		haveCandidate = true
	}
	if contextCount != 4 || !haveContext || !haveCandidate {
		t.Fatalf("first sources = %d total, context=%d, want four context chunks and a candidate", len(first.Sources), contextCount)
	}
	if candidate.NoteID == contextSource.NoteID || candidate.SourceID == contextSource.SourceID {
		t.Fatalf("selected sources = %+v and %+v, want distinct notes", candidate, contextSource)
	}
	candidateWords := strings.Fields(candidate.Passage)
	contextWords := strings.Fields(contextSource.Passage)
	if len(candidateWords) == 0 || len(contextWords) == 0 {
		t.Fatalf("selected sources have empty passages: candidate=%+v context=%+v", candidate, contextSource)
	}
	candidatePassage := candidateWords[0]
	contextPassage := contextWords[0]
	if len(candidatePassage) >= 1000 || len(contextPassage) >= 1000 {
		t.Fatalf("selected evidence markers are too long: candidate=%d context=%d", len(candidatePassage), len(contextPassage))
	}

	secondMock := &transport.MockTransport{}
	secondMock.EnqueueResponses(associationResponse(
		t,
		candidate.SourceID, candidatePassage,
		contextSource.SourceID, contextPassage,
	))
	second, err := discovery.Discover(context.Background(), discovery.Config{
		DB:          f.db,
		Executor:    newExecutor(secondMock),
		ReadOnlyCLI: true,
	}, discovery.Request{
		Query:             "candidate",
		ProjectOverviewID: f.overview,
		Limit:             1,
	})
	if err != nil {
		t.Fatalf("second Discover: %v", err)
	}
	if len(second.Candidates) != 1 || second.Candidates[0].SourceID != candidate.SourceID {
		t.Fatalf("second candidates = %+v, want grounded candidate %s", second.Candidates, candidate.SourceID)
	}
	if len(second.Candidates[0].Evidence) != 2 {
		t.Fatalf("second evidence = %+v, want candidate and context citations", second.Candidates[0].Evidence)
	}
}

func TestDiscoverReportsProviderAndOversizeErrors(t *testing.T) {
	t.Run("missing fixed skill", func(t *testing.T) {
		f := newDiscoveryFixture(t)
		mock := &transport.MockTransport{}
		executor := newExecutor(mock)
		delete(executor.Skills, "discover-explain-association")
		_, err := discovery.Discover(context.Background(), discovery.Config{
			DB:          f.db,
			Executor:    executor,
			ReadOnlyCLI: true,
		}, discovery.Request{Query: "lantern"})
		if !errors.Is(err, discovery.ErrProviderUnavailable) {
			t.Fatalf("err = %v, want ErrProviderUnavailable", err)
		}
	})

	t.Run("oversize association response", func(t *testing.T) {
		f := newDiscoveryFixture(t)
		mock := &transport.MockTransport{}
		mock.EnqueueResponses(strings.Repeat("x", 24001))
		_, err := discoverRequest(f, mock)
		if !errors.Is(err, discovery.ErrMalformedOutput) {
			t.Fatalf("err = %v, want ErrMalformedOutput", err)
		}
	})
}

func TestDiscoverBoundsHugeUTF8Passage(t *testing.T) {
	f := newDiscoveryFixture(t, map[string]string{
		"notes/huge.md": "needle " + strings.Repeat("界", 3000),
	})
	mock := &transport.MockTransport{}
	mock.EnqueueResponses(`{"connections":[]}`)
	result, err := discovery.Discover(context.Background(), discovery.Config{
		DB:          f.db,
		Executor:    newExecutor(mock),
		ReadOnlyCLI: true,
	}, discovery.Request{Query: "needle", Limit: 1})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	var huge *discovery.Source
	for i := range result.Sources {
		if result.Sources[i].NoteID == "notes/huge.md" {
			huge = &result.Sources[i]
			break
		}
	}
	if huge == nil {
		t.Fatalf("sources = %+v, want huge source", result.Sources)
	}
	if len(huge.Passage) > 2000 || !utf8.ValidString(huge.Passage) {
		t.Fatalf("huge passage bytes=%d valid=%v, want <=2000 valid UTF-8", len(huge.Passage), utf8.ValidString(huge.Passage))
	}
	if !strings.HasPrefix(huge.Passage, "needle ") || !huge.Truncated {
		t.Fatalf("huge source = %+v, want exact bounded prefix and Truncated=true", huge)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("candidates = %+v, want empty for empty association response", result.Candidates)
	}
}
