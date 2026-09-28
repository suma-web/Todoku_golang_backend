package schoolpost

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"todoku_golang_backend/internal/database"
	"todoku_golang_backend/internal/notification"
)

// Run against a disposable PostgreSQL database using TEST_DATABASE_URL.
// Each run owns a separate schema; existing application tables are untouched.
func TestNotificationIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration test")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	schema := fmt.Sprintf("notification_test_%d", time.Now().UnixNano())
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE SCHEMA ` + schema)
	defer db.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	exec(`SET search_path TO ` + schema)
	if err := database.Migrate(ctx, db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO users(id,name,email,password_hash,role,is_active) VALUES
 (1,'Teacher','teacher@test','x','teacher',true),(2,'Target','target@test','x','student',true),
 (3,'Outside','outside@test','x','student',true),(4,'Inactive','inactive@test','x','student',false),
 (5,'Deleted','deleted@test','x','student',true),(6,'Admin','admin@test','x','admin',true)`)
	exec(`UPDATE users SET deleted_at=NOW() WHERE id=5`)
	exec(`INSERT INTO school_groups(id,name,type) VALUES(1,'A','class'),(2,'Club','club'),(3,'B','class')`)
	exec(`INSERT INTO user_school_groups VALUES(1,1),(2,1),(2,2),(3,3),(4,1),(5,1),(6,1)`)
	repo := NewRepository(db)
	service := NewService(repo)
	notices := notification.NewRepository(db)
	input := Post{Title: "Before", Content: "Content", Type: "notice", Priority: "normal", GroupIDs: []int64{1, 2}}
	post, err := service.Create(ctx, 1, input)
	if err != nil {
		t.Fatal(err)
	}
	check := func(uid int64, count int) notification.List {
		t.Helper()
		got, err := notices.List(ctx, uid)
		if err != nil {
			t.Fatal(err)
		}
		if got.UnreadCount != count {
			t.Fatalf("user %d unread=%d want %d", uid, got.UnreadCount, count)
		}
		return got
	}
	input.Title = "Normal"
	if _, err = service.Update(ctx, post.ID, 1, input); err != nil {
		t.Fatal(err)
	}
	check(2, 0)

	if err = service.MarkRead(ctx, post.ID, 2); err != nil {
		t.Fatal(err)
	}
	input.Notify = true
	input.ChangeSummary = "集合時間を変更"
	input.Title = "Notified"
	updated, err := service.Update(ctx, post.ID, 1, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID == post.ID || updated.PreviousPostID == nil || *updated.PreviousPostID != post.ID {
		t.Fatal("repost must preserve source and have new id")
	}
	old, err := service.Get(ctx, post.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if old.Title != "Normal" || !old.ReadByMe || !old.Superseded || old.LatestPostID == nil || *old.LatestPostID != updated.ID {
		t.Fatalf("source changed or latest link missing: %#v", old)
	}
	fresh, err := service.Get(ctx, updated.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.ReadByMe || !fresh.CanViewPrevious || fresh.ChangeSummary != input.ChangeSummary {
		t.Fatalf("new post should be unread with history: %#v", fresh)
	}
	got := check(2, 1)
	check(1, 0)
	check(3, 0)
	check(4, 0)
	check(5, 0)
	if len(got.Items) != 1 || got.Items[0].PostID != updated.ID {
		t.Fatal("notification must target new post once")
	}
	for _, uid := range []int64{1, 3, 4, 5} {
		var stored int
		if err := db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE recipient_user_id=$1`, uid).Scan(&stored); err != nil || stored != 0 {
			t.Fatalf("excluded recipient %d: %d %v", uid, stored, err)
		}
	}
	id := got.Items[0].ID
	if ok, err := notices.MarkRead(ctx, id, 3); err != nil || ok {
		t.Fatal("other user marked notification read")
	}
	for i := 0; i < 2; i++ {
		if ok, err := notices.MarkRead(ctx, id, 2); err != nil || !ok {
			t.Fatal("read failed")
		}
	}
	check(2, 0)
	// A retry against the same source cannot create a branch or duplicate notification.
	if _, err = service.Update(ctx, post.ID, 1, input); err != ErrConflict {
		t.Fatalf("old repost: %v", err)
	}
	ordinary := input
	ordinary.Notify = false
	if _, err = service.Update(ctx, post.ID, 1, ordinary); err != ErrConflict {
		t.Fatalf("old edit: %v", err)
	}
	if err = service.Delete(ctx, post.ID, 1); err != ErrConflict {
		t.Fatalf("old delete: %v", err)
	}
	if err = service.Delete(ctx, updated.ID, 1); err != ErrConflict {
		t.Fatalf("history delete: %v", err)
	}
	newest, err := service.Update(ctx, updated.ID, 6, input)
	if err != nil {
		t.Fatal(err)
	}
	check(2, 1)
	check(6, 1) // Editor excluded, original author retained.
	old, err = service.Get(ctx, post.ID, 2)
	if err != nil || old.LatestPostID == nil || *old.LatestPostID != newest.ID {
		t.Fatal("multi-hop latest link failed")
	}
	// Failed notification insert must leave no new version and keep source editable.
	exec(`CREATE FUNCTION reject_notification() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$`)
	exec(`CREATE TRIGGER reject_notification BEFORE INSERT ON notifications FOR EACH ROW EXECUTE FUNCTION reject_notification()`)
	if _, err = service.Update(ctx, newest.ID, 1, input); err == nil {
		t.Fatal("expected insertion failure")
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM school_posts WHERE previous_post_id=$1`, newest.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed repost left a version")
	}
	exec(`DROP TRIGGER reject_notification ON notifications`)
	if _, err = service.Update(ctx, newest.ID, 3, input); err != ErrNotFound {
		t.Fatalf("unauthorized: %v", err)
	}
	input.GroupIDs = []int64{3}
	last, err := service.Update(ctx, newest.ID, 1, input)
	if err != nil {
		t.Fatal(err)
	}
	check(3, 1)
	fresh, err = service.Get(ctx, last.ID, 3)
	if err != nil || fresh.CanViewPrevious {
		t.Fatal("new recipient must not gain old audience permissions")
	}
	if _, err = service.Get(ctx, newest.ID, 3); err != ErrNotFound {
		t.Fatal("old content leaked")
	}
	old, err = service.Get(ctx, post.ID, 2)
	if err != nil || old.LatestPostID != nil {
		t.Fatal("inaccessible latest link leaked")
	}
	timeline, err := service.Timeline(ctx, 3)
	if err != nil || len(timeline) != 1 || timeline[0].ID != last.ID {
		t.Fatalf("latest timeline=%#v err=%v", timeline, err)
	}
	timeline, err = service.Timeline(ctx, 2)
	if err != nil || len(timeline) != 0 {
		t.Fatalf("superseded posts in timeline: %#v %v", timeline, err)
	}
	exec(`UPDATE school_posts SET expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, last.ID)
	check(3, 0)
}
