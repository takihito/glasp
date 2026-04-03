package history

import (
	"sync"
	"testing"
)

func TestAppendAssignsMaxIDPlusOne(t *testing.T) {
	root := t.TempDir()
	if err := Append(root, Entry{ID: 4, Command: "pull", Status: "success"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if err := Append(root, Entry{Command: "push", Status: "error"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}

	entries, err := Read(root, ReadOptions{Order: "asc"})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[1].ID != 5 {
		t.Fatalf("expected auto id 5, got %d", entries[1].ID)
	}
}

func TestReadFiltersAndSorts(t *testing.T) {
	root := t.TempDir()
	seed := []Entry{
		{ID: 1, Command: "pull", Status: "success"},
		{ID: 2, Command: "push", Status: "error"},
		{ID: 3, Command: "pull", Status: "error"},
	}
	for _, e := range seed {
		if err := Append(root, e); err != nil {
			t.Fatalf("append failed: %v", err)
		}
	}

	desc, err := Read(root, ReadOptions{Order: "desc"})
	if err != nil {
		t.Fatalf("read desc failed: %v", err)
	}
	if len(desc) != 3 || desc[0].ID != 3 || desc[2].ID != 1 {
		t.Fatalf("unexpected desc order: %#v", desc)
	}

	filtered, err := Read(root, ReadOptions{Order: "asc", Status: "error", Command: "pull"})
	if err != nil {
		t.Fatalf("read filtered failed: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != 3 {
		t.Fatalf("unexpected filtered result: %#v", filtered)
	}
}

func TestReadMissingFileReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	entries, err := Read(root, ReadOptions{})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty entries, got %d", len(entries))
	}
}

func TestAppendConcurrentWritersAssignUniqueIDs(t *testing.T) {
	root := t.TempDir()
	const writers = 40

	var wg sync.WaitGroup
	errCh := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Append(root, Entry{Command: "pull", Status: "success"}); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("append failed: %v", err)
		}
	}

	entries, err := Read(root, ReadOptions{Order: "asc"})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if len(entries) != writers {
		t.Fatalf("expected %d entries, got %d", writers, len(entries))
	}
	seen := make(map[int64]struct{}, writers)
	for _, entry := range entries {
		if _, ok := seen[entry.ID]; ok {
			t.Fatalf("duplicate id detected: %d", entry.ID)
		}
		seen[entry.ID] = struct{}{}
	}
}

func TestGetByID(t *testing.T) {
	root := t.TempDir()
	if err := Append(root, Entry{ID: 10, Command: "push", Status: "success"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if err := Append(root, Entry{ID: 11, Command: "pull", Status: "error"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}

	entry, found, err := GetByID(root, 11)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if !found {
		t.Fatalf("expected entry to be found")
	}
	if entry.Command != "pull" {
		t.Fatalf("expected command pull, got %q", entry.Command)
	}

	_, found, err = GetByID(root, 999)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if found {
		t.Fatalf("expected entry not to be found")
	}
}
