package records_test

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/nightshiftco/nightshift/internal/records"
)

// openMemoryStore returns a fresh in-memory [records.SQLite] for use
// inside an example. Each example passes a unique name so the shared
// cache key does not collide across examples.
func openMemoryStore(name string) records.RecordStore {
	s, err := records.OpenSQLite("file:" + name + "?mode=memory&cache=shared")
	if err != nil {
		panic(err)
	}
	return s
}

// Example_quickstart shows the minimal Put/Get cycle.
func Example_quickstart() {
	store := openMemoryStore("quickstart")
	defer store.Close()

	ctx := context.Background()

	out, err := store.Put(ctx, records.Record{
		Collection:  "schedules",
		Key:         "sch_a",
		Attributes:  map[string]string{"user_id": "u_1"},
		ContentType: "application/json",
		Data:        []byte(`{"cron":"0 * * * *"}`),
	}, nil, "")
	if err != nil {
		fmt.Println("put:", err)
		return
	}
	fmt.Println("written version:", out.Version)

	got, err := store.Get(ctx, "schedules", "sch_a")
	if err != nil {
		fmt.Println("get:", err)
		return
	}
	fmt.Println("data:", string(got.Data))
	fmt.Println("user_id:", got.Attributes["user_id"])
	// Output:
	// written version: 1
	// data: {"cron":"0 * * * *"}
	// user_id: u_1
}

// ExampleRecordStore_Put_optimisticConcurrency demonstrates ifVersion
// semantics: pass &0 to require "must not exist", pass &n to require
// the current version equal n. A mismatch returns [records.ErrVersionConflict].
func ExampleRecordStore_Put_optimisticConcurrency() {
	store := openMemoryStore("optimistic")
	defer store.Close()

	ctx := context.Background()
	zero := int64(0)

	first, err := store.Put(ctx, records.Record{
		Collection: "runs",
		Key:        "run_42",
		Data:       []byte("queued"),
	}, &zero, "")
	if err != nil {
		fmt.Println("first:", err)
		return
	}
	fmt.Println("created at version:", first.Version)

	stale := int64(99)
	_, err = store.Put(ctx, records.Record{
		Collection: "runs",
		Key:        "run_42",
		Data:       []byte("running"),
	}, &stale, "")
	if errors.Is(err, records.ErrVersionConflict) {
		fmt.Println("stale write rejected")
	}

	updated, err := store.Put(ctx, records.Record{
		Collection: "runs",
		Key:        "run_42",
		Data:       []byte("running"),
	}, &first.Version, "")
	if err != nil {
		fmt.Println("update:", err)
		return
	}
	fmt.Println("updated to version:", updated.Version)
	// Output:
	// created at version: 1
	// stale write rejected
	// updated to version: 2
}

// ExampleRecordStore_Put_idempotency shows that a repeat Put with the
// same idempotency key returns the original write without bumping the
// version.
func ExampleRecordStore_Put_idempotency() {
	store := openMemoryStore("idempotent")
	defer store.Close()

	ctx := context.Background()

	rec := records.Record{
		Collection: "billing_events",
		Key:        "evt_1",
		Data:       []byte(`{"amount":100}`),
	}

	a, _ := store.Put(ctx, rec, nil, "client-token-xyz")
	b, _ := store.Put(ctx, rec, nil, "client-token-xyz")

	fmt.Println("first version:", a.Version)
	fmt.Println("replay version:", b.Version)
	// Output:
	// first version: 1
	// replay version: 1
}

// ExampleRecordStore_List_byAttribute filters a collection by an
// indexed attribute. Pass a string like "created_at asc" via OrderBy
// for stable iteration.
func ExampleRecordStore_List_byAttribute() {
	store := openMemoryStore("list")
	defer store.Close()

	ctx := context.Background()
	for _, key := range []string{"sch_a", "sch_b", "sch_c"} {
		userID := "u_1"
		if key == "sch_b" {
			userID = "u_2"
		}
		_, _ = store.Put(ctx, records.Record{
			Collection: "schedules",
			Key:        key,
			Attributes: map[string]string{"user_id": userID},
		}, nil, "")
	}

	page, _, err := store.List(ctx, records.ListQuery{
		Collection:       "schedules",
		AttributeFilters: map[string]string{"user_id": "u_1"},
		OrderBy:          "created_at asc",
	})
	if err != nil {
		fmt.Println("list:", err)
		return
	}

	keys := make([]string, len(page))
	for i, r := range page {
		keys[i] = r.Key
	}
	sort.Strings(keys) // created_at can tie at ns precision; sort for determinism.
	for _, k := range keys {
		fmt.Println(k)
	}
	// Output:
	// sch_a
	// sch_c
}

// ExampleRecordStore_Delete_versioned shows that Delete honors
// ifVersion the same way Put does. Deleting a missing record returns
// [records.ErrNotFound].
func ExampleRecordStore_Delete_versioned() {
	store := openMemoryStore("delete")
	defer store.Close()

	ctx := context.Background()
	r, _ := store.Put(ctx, records.Record{
		Collection: "schedules",
		Key:        "sch_a",
	}, nil, "")

	stale := int64(99)
	if err := store.Delete(ctx, "schedules", "sch_a", &stale); errors.Is(err, records.ErrVersionConflict) {
		fmt.Println("stale delete rejected")
	}

	if err := store.Delete(ctx, "schedules", "sch_a", &r.Version); err == nil {
		fmt.Println("deleted")
	}

	if _, err := store.Get(ctx, "schedules", "sch_a"); errors.Is(err, records.ErrNotFound) {
		fmt.Println("gone")
	}
	// Output:
	// stale delete rejected
	// deleted
	// gone
}
