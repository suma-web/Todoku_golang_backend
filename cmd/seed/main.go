// Command seed inserts development-only data for the documented school demo.
// It is never called by the API server and requires DEMO_SEED=true.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"twitter_golang_backend/internal/database"
	"twitter_golang_backend/internal/schoolpost"
	schoolsearch "twitter_golang_backend/internal/search"
)

const defaultDevelopmentDatabaseURL = "postgres://twitter:twitter_password@localhost:5432/twitter?sslmode=disable"

type demoUser struct {
	Key, Name, Email, Password, Role string
}

type demoGroup struct {
	Name, Type string
}

type demoPost struct {
	Key, Author, Title, Content, Priority string
	Groups                                []string
	CreatedAt                             time.Time
}

type demoQuestion struct {
	Key, Author, Category, Title, Content, Visibility, Status string
	Answerer, Answer                                          string
	CreatedAt                                                 time.Time
}

var users = []demoUser{
	{Key: "admin", Name: "【デモ】学校管理者", Email: "demo.admin@school.local", Password: "DemoAdmin@2026", Role: "admin"},
	{Key: "tanaka", Name: "【デモ】田中先生", Email: "demo.tanaka@school.local", Password: "DemoTanaka@2026", Role: "teacher"},
	{Key: "sato", Name: "【デモ】佐藤先生", Email: "demo.sato@school.local", Password: "DemoSato@2026", Role: "teacher"},
	{Key: "suzuki", Name: "【デモ】鈴木先生", Email: "demo.suzuki@school.local", Password: "DemoSuzuki@2026", Role: "teacher"},
	{Key: "yamada", Name: "【デモ】山田太郎", Email: "demo.yamada@school.local", Password: "DemoYamada@2026", Role: "student"},
	{Key: "sasaki", Name: "【デモ】佐々木花子", Email: "demo.sasaki@school.local", Password: "DemoSasaki@2026", Role: "student"},
	{Key: "takahashi", Name: "【デモ】高橋健", Email: "demo.takahashi@school.local", Password: "DemoTakahashi@2026", Role: "student"},
}

var groups = []demoGroup{
	{Name: "2年", Type: "grade"}, {Name: "3年", Type: "grade"},
	{Name: "2年A組", Type: "class"}, {Name: "2年B組", Type: "class"}, {Name: "3年B組", Type: "class"},
	{Name: "サッカー部", Type: "club"}, {Name: "バスケットボール部", Type: "club"},
	{Name: "文化祭委員会", Type: "committee"}, {Name: "生徒会", Type: "committee"},
	{Name: "数学科", Type: "department"}, {Name: "進路指導部", Type: "department"}, {Name: "ICT担当", Type: "department"},
}

var memberships = map[string][]string{
	"tanaka":    {"2年A組", "数学科"},
	"sato":      {"進路指導部"},
	"suzuki":    {"ICT担当", "サッカー部"},
	"yamada":    {"2年", "2年A組", "サッカー部"},
	"sasaki":    {"2年", "2年A組", "文化祭委員会"},
	"takahashi": {"3年", "3年B組", "サッカー部"},
}

var categoryGroups = map[string]string{
	"数学の授業": "数学科", "進路・奨学金": "進路指導部", "ICT・端末": "ICT担当", "部活動": "サッカー部",
}

var posts = []demoPost{
	{Key: "math", Author: "tanaka", Title: "来週の数学小テストについて", Content: "来週月曜日の数学の授業で小テストを実施します。範囲は教科書42〜55ページです。", Priority: "normal", Groups: []string{"2年A組"}, CreatedAt: mustTime("2026-08-25T09:00:00+09:00")},
	{Key: "career", Author: "sato", Title: "進路希望調査の提出について", Content: "進路希望調査の提出期限は9月5日です。期限までに必ず提出してください。", Priority: "important", Groups: []string{"2年"}, CreatedAt: mustTime("2026-08-26T09:00:00+09:00")},
	{Key: "soccer", Author: "suzuki", Title: "土曜日の練習時間変更", Content: "土曜日の練習開始時刻を9:00から10:00へ変更します。", Priority: "important", Groups: []string{"サッカー部"}, CreatedAt: mustTime("2026-08-27T09:00:00+09:00")},
	// Search fixture: the requested career notice does not contain the word 奨学金.
	{Key: "scholarship", Author: "sato", Title: "奨学金説明会のお知らせ", Content: "進路指導部から、奨学金制度と申請方法を説明します。", Priority: "normal", Groups: []string{"2年"}, CreatedAt: mustTime("2026-08-24T09:00:00+09:00")},
}

var questions = []demoQuestion{
	{Key: "math", Author: "yamada", Category: "数学の授業", Title: "小テストに二次関数は含まれますか？", Content: "教科書55ページまでとのことですが、二次関数の応用問題も出題範囲でしょうか。", Visibility: "public", Status: "answered", Answerer: "tanaka", Answer: "二次関数の基本問題までは範囲に含みますが、応用問題は今回は出題しません。", CreatedAt: mustTime("2026-08-27T15:00:00+09:00")},
	{Key: "career-private", Author: "sasaki", Category: "進路・奨学金", Title: "奨学金について相談したい", Content: "家庭の事情も含めて相談したいのですが、利用できる奨学金について教えてください。", Visibility: "private", Status: "open", CreatedAt: mustTime("2026-08-28T10:00:00+09:00")},
	{Key: "ict", Author: "takahashi", Category: "ICT・端末", Title: "学校端末からWi-Fiに接続できません", Content: "学校端末で校内Wi-Fiを選んでも接続できません。確認方法を教えてください。", Visibility: "public", Status: "resolved", Answerer: "suzuki", Answer: "端末を再起動し、校内ネットワーク設定を一度削除してから再接続してください。", CreatedAt: mustTime("2026-08-26T14:00:00+09:00")},
	// Public Q&A fixture so a student search for 奨学金 returns a Q&A without exposing the private consultation.
	{Key: "scholarship-public", Author: "yamada", Category: "進路・奨学金", Title: "奨学金の募集時期を知りたい", Content: "校内で案内される奨学金の募集時期はいつ頃ですか。", Visibility: "public", Status: "answered", Answerer: "sato", Answer: "主な募集は9月から始まります。詳細は進路指導部のお知らせを確認してください。", CreatedAt: mustTime("2026-08-23T14:00:00+09:00")},
}

func main() {
	if os.Getenv("DEMO_SEED") != "true" {
		log.Fatal("refusing to seed: set DEMO_SEED=true explicitly")
	}
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") {
		log.Fatal("refusing to seed when APP_ENV=production")
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = defaultDevelopmentDatabaseURL
	}

	db, err := database.Open(databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, db, "migrations"); err != nil {
		log.Fatal(err)
	}
	if err := seed(ctx, db); err != nil {
		log.Fatal(err)
	}
	if err := verify(ctx, db); err != nil {
		log.Fatal(err)
	}

	fmt.Println("デモデータを投入・検証しました。再実行しても同じデモデータを更新します。")
	fmt.Println("ログイン情報:")
	for _, user := range users {
		fmt.Printf("- %s (%s): %s / %s\n", user.Name, user.Role, user.Email, user.Password)
	}
}

func seed(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	userIDs, err := seedUsers(ctx, tx)
	if err != nil {
		return err
	}
	groupIDs, err := seedGroups(ctx, tx)
	if err != nil {
		return err
	}
	if err := seedMemberships(ctx, tx, userIDs, groupIDs); err != nil {
		return err
	}
	categoryIDs, err := seedCategories(ctx, tx, groupIDs)
	if err != nil {
		return err
	}
	postIDs, err := seedPosts(ctx, tx, userIDs, groupIDs)
	if err != nil {
		return err
	}
	if err := seedStatuses(ctx, tx, postIDs, userIDs); err != nil {
		return err
	}
	if err := seedQuestions(ctx, tx, userIDs, categoryIDs); err != nil {
		return err
	}
	return tx.Commit()
}

func seedUsers(ctx context.Context, tx *sql.Tx) (map[string]int64, error) {
	ids := map[string]int64{}
	for _, user := range users {
		hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		var id int64
		err = tx.QueryRowContext(ctx, `INSERT INTO users(name,email,birthday,password_hash,role,is_active)
			VALUES($1,$2,DATE '2000-01-01',$3,$4,TRUE)
			ON CONFLICT(email) DO UPDATE SET name=EXCLUDED.name,password_hash=EXCLUDED.password_hash,role=EXCLUDED.role,is_active=TRUE,updated_at=NOW()
			RETURNING id`, user.Name, user.Email, string(hash), user.Role).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("seed user %s: %w", user.Key, err)
		}
		ids[user.Key] = id
	}
	return ids, nil
}

func seedGroups(ctx context.Context, tx *sql.Tx) (map[string]int64, error) {
	ids := map[string]int64{}
	for _, group := range groups {
		var id int64
		err := tx.QueryRowContext(ctx, `INSERT INTO school_groups(name,type) VALUES($1,$2)
			ON CONFLICT(name,type) DO UPDATE SET name=EXCLUDED.name RETURNING id`, group.Name, group.Type).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("seed group %s: %w", group.Name, err)
		}
		ids[group.Name] = id
	}
	return ids, nil
}

func seedMemberships(ctx context.Context, tx *sql.Tx, userIDs, groupIDs map[string]int64) error {
	for _, userID := range userIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM user_school_groups WHERE user_id=$1`, userID); err != nil {
			return fmt.Errorf("reset demo memberships: %w", err)
		}
	}
	for userKey, names := range memberships {
		for _, name := range names {
			if _, err := tx.ExecContext(ctx, `INSERT INTO user_school_groups(user_id,group_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, userIDs[userKey], groupIDs[name]); err != nil {
				return fmt.Errorf("seed membership %s/%s: %w", userKey, name, err)
			}
		}
	}
	return nil
}

func seedCategories(ctx context.Context, tx *sql.Tx, groupIDs map[string]int64) (map[string]int64, error) {
	ids := map[string]int64{}
	for category, group := range categoryGroups {
		var id int64
		err := tx.QueryRowContext(ctx, `INSERT INTO question_categories(name,group_id) VALUES($1,$2)
			ON CONFLICT(name) DO UPDATE SET group_id=EXCLUDED.group_id RETURNING id`, category, groupIDs[group]).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("seed category %s: %w", category, err)
		}
		ids[category] = id
	}
	return ids, nil
}

func seedPosts(ctx context.Context, tx *sql.Tx, userIDs, groupIDs map[string]int64) (map[string]int64, error) {
	ids := map[string]int64{}
	for _, post := range posts {
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT p.id FROM school_posts p WHERE p.author_id=$1 AND p.title=$2 ORDER BY p.id LIMIT 1`, userIDs[post.Author], post.Title).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `INSERT INTO school_posts(author_id,type,title,content,priority,created_at,updated_at) VALUES($1,'notice',$2,$3,$4,$5,$5) RETURNING id`, userIDs[post.Author], post.Title, post.Content, post.Priority, post.CreatedAt).Scan(&id)
		} else if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE school_posts SET author_id=$2,type='notice',content=$3,priority=$4,expires_at=NULL,created_at=$5,updated_at=$5 WHERE id=$1`, id, userIDs[post.Author], post.Content, post.Priority, post.CreatedAt)
		}
		if err != nil {
			return nil, fmt.Errorf("seed post %s: %w", post.Key, err)
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM school_post_groups WHERE post_id=$1`, id); err != nil {
			return nil, err
		}
		for _, group := range post.Groups {
			if _, err = tx.ExecContext(ctx, `INSERT INTO school_post_groups(post_id,group_id) VALUES($1,$2)`, id, groupIDs[group]); err != nil {
				return nil, err
			}
		}
		ids[post.Key] = id
	}
	return ids, nil
}

func seedStatuses(ctx context.Context, tx *sql.Tx, postIDs, userIDs map[string]int64) error {
	for _, key := range []string{"career", "soccer"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM school_post_statuses WHERE post_id=$1`, postIDs[key]); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO school_post_statuses(post_id,user_id,read_at) VALUES
		($1,$2,TIMESTAMPTZ '2026-08-28 09:00:00+09'),
		($1,$3,TIMESTAMPTZ '2026-08-28 10:00:00+09'),
		($4,$2,TIMESTAMPTZ '2026-08-28 11:00:00+09')`,
		postIDs["career"], userIDs["yamada"], userIDs["sasaki"], postIDs["soccer"])
	return err
}

func seedQuestions(ctx context.Context, tx *sql.Tx, userIDs, categoryIDs map[string]int64) error {
	for _, question := range questions {
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM questions WHERE user_id=$1 AND title=$2 ORDER BY id LIMIT 1`, userIDs[question.Author], question.Title).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `INSERT INTO questions(user_id,category_id,title,content,visibility,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$7) RETURNING id`, userIDs[question.Author], categoryIDs[question.Category], question.Title, question.Content, question.Visibility, question.Status, question.CreatedAt).Scan(&id)
		} else if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE questions SET category_id=$2,content=$3,visibility=$4,status=$5,created_at=$6,updated_at=$6 WHERE id=$1`, id, categoryIDs[question.Category], question.Content, question.Visibility, question.Status, question.CreatedAt)
		}
		if err != nil {
			return fmt.Errorf("seed question %s: %w", question.Key, err)
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM question_answers WHERE question_id=$1`, id); err != nil {
			return err
		}
		if question.Answer != "" {
			if _, err = tx.ExecContext(ctx, `INSERT INTO question_answers(question_id,user_id,content,created_at) VALUES($1,$2,$3,$4::timestamptz + INTERVAL '1 hour')`, id, userIDs[question.Answerer], question.Answer, question.CreatedAt); err != nil {
				return err
			}
		}
	}
	return nil
}

func verify(ctx context.Context, db *sql.DB) error {
	checks := []struct {
		name, query string
		want        int
	}{
		{"demo users", `SELECT COUNT(*) FROM users WHERE email LIKE 'demo.%@school.local'`, len(users)},
		{"groups", `SELECT COUNT(*) FROM school_groups WHERE (name,type) IN (('2年','grade'),('3年','grade'),('2年A組','class'),('2年B組','class'),('3年B組','class'),('サッカー部','club'),('バスケットボール部','club'),('文化祭委員会','committee'),('生徒会','committee'),('数学科','department'),('進路指導部','department'),('ICT担当','department'))`, len(groups)},
		{"categories", `SELECT COUNT(*) FROM question_categories WHERE name IN ('数学の授業','進路・奨学金','ICT・端末','部活動')`, len(categoryGroups)},
		{"private visible to owner", accessCountSQL("demo.sasaki@school.local", "奨学金について相談したい"), 1},
		{"private visible to assigned teacher", accessCountSQL("demo.sato@school.local", "奨学金について相談したい"), 1},
		{"private visible to admin", accessCountSQL("demo.admin@school.local", "奨学金について相談したい"), 1},
		{"private hidden from other student", accessCountSQL("demo.yamada@school.local", "奨学金について相談したい"), 0},
		{"private hidden from unrelated teacher", accessCountSQL("demo.tanaka@school.local", "奨学金について相談したい"), 0},
		{"soccer visible to Yamada and Takahashi only among demo students", `SELECT COUNT(DISTINCT u.id) FROM users u JOIN user_school_groups ug ON ug.user_id=u.id JOIN school_post_groups pg ON pg.group_id=ug.group_id JOIN school_posts p ON p.id=pg.post_id WHERE p.title='土曜日の練習時間変更' AND u.role='student' AND u.email LIKE 'demo.%@school.local'`, 2},
	}
	for _, check := range checks {
		var got int
		if err := db.QueryRowContext(ctx, check.query).Scan(&got); err != nil {
			return fmt.Errorf("verify %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("verify %s: got %d, want %d", check.name, got, check.want)
		}
	}
	if err := verifyStatuses(ctx, db); err != nil {
		return err
	}
	if err := verifySearch(ctx, db); err != nil {
		return err
	}
	return nil
}

func verifyStatuses(ctx context.Context, db *sql.DB) error {
	repository := schoolpost.NewRepository(db)
	expected := map[string]schoolpost.Status{
		"進路希望調査の提出について": {TargetCount: 2, ReadCount: 2, UnreadCount: 0},
		"土曜日の練習時間変更":    {TargetCount: 2, ReadCount: 1, UnreadCount: 1},
	}
	for title, want := range expected {
		var postID int64
		if err := db.QueryRowContext(ctx, `SELECT id FROM school_posts WHERE title=$1 ORDER BY id LIMIT 1`, title).Scan(&postID); err != nil {
			return fmt.Errorf("verify status post %s: %w", title, err)
		}
		got, err := repository.Status(ctx, postID)
		if err != nil {
			return fmt.Errorf("verify status %s: %w", title, err)
		}
		if got.TargetCount != want.TargetCount || got.ReadCount != want.ReadCount || got.UnreadCount != want.UnreadCount {
			return fmt.Errorf("verify status %s: got %+v, want %+v", title, got, want)
		}
	}
	return nil
}

func verifySearch(ctx context.Context, db *sql.DB) error {
	var userID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE email='demo.yamada@school.local'`).Scan(&userID); err != nil {
		return err
	}
	repository := schoolsearch.NewRepository(db)
	wants := map[string]map[string]bool{
		"奨学金": {"post": true, "question": true, "contact": true},
		"数学":  {"post": true, "question": true, "contact": true},
		"ICT": {"question": true, "contact": true},
	}
	for query, types := range wants {
		results, err := repository.Search(ctx, userID, query)
		if err != nil {
			return fmt.Errorf("verify search %s: %w", query, err)
		}
		found := map[string]bool{}
		for _, result := range results {
			found[result.Type] = true
			if result.Title == "奨学金について相談したい" {
				return fmt.Errorf("verify search %s: private question was exposed", query)
			}
		}
		for resultType := range types {
			if !found[resultType] {
				return fmt.Errorf("verify search %s: missing %s result", query, resultType)
			}
		}
	}
	return nil
}

func accessCountSQL(email, title string) string {
	return fmt.Sprintf(`SELECT COUNT(*) FROM questions q JOIN question_categories c ON c.id=q.category_id JOIN users viewer ON viewer.email='%s' WHERE q.title='%s' AND(q.user_id=viewer.id OR q.visibility='public' OR viewer.role='admin' OR(viewer.role='teacher' AND EXISTS(SELECT 1 FROM user_school_groups ug WHERE ug.user_id=viewer.id AND ug.group_id=c.group_id)))`, email, title)
}

func mustTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}
