package message

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"twitter_golang_backend/internal/auth"
)

type Handler struct{ repository *Repository }

func NewHandler(repository *Repository) *Handler { return &Handler{repository: repository} }

func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "ログインが必要です")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var request CreateGroupRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "リクエストの形式が正しくありません")
		return
	}
	name := strings.TrimSpace(request.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		writeError(w, http.StatusBadRequest, "グループ名は1文字以上100文字以内で入力してください")
		return
	}
	members := uniqueNames(request.MemberNames)
	if len(members) > 50 {
		writeError(w, http.StatusBadRequest, "追加できるメンバーは50人までです")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	group, err := h.repository.CreateGroup(ctx, userID, name, members)
	if errors.Is(err, ErrUserNotFound) {
		writeError(w, http.StatusBadRequest, "指定されたユーザーが見つかりません")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "グループを作成できませんでした")
		return
	}
	writeJSON(w, http.StatusCreated, group)
}

func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "ログインが必要です")
		return
	}
	limit, offset, ok := pagination(w, r, 20)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	groups, err := h.repository.ListGroups(ctx, userID, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "グループ一覧を取得できませんでした")
		return
	}
	hasMore := len(groups) > limit
	if hasMore {
		groups = groups[:limit]
	}
	writeJSON(w, http.StatusOK, GroupListResponse{Groups: groups, Limit: limit, Offset: offset, HasMore: hasMore})
}

func (h *Handler) GetGroup(w http.ResponseWriter, r *http.Request) {
	groupID, userID, ok := requestIDs(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	group, err := h.repository.FindGroup(ctx, groupID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "グループが見つかりません")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "グループを取得できませんでした")
		return
	}
	writeJSON(w, http.StatusOK, group)
}

func (h *Handler) CreateMessage(w http.ResponseWriter, r *http.Request) {
	groupID, userID, ok := requestIDs(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var request CreateMessageRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "リクエストの形式が正しくありません")
		return
	}
	text := strings.TrimSpace(request.Message)
	if text == "" || utf8.RuneCountInString(text) > 1000 {
		writeError(w, http.StatusBadRequest, "メッセージは1文字以上1000文字以内で入力してください")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if !h.requireMember(w, ctx, groupID, userID, "このグループにはメッセージを投稿できません") {
		return
	}
	created, err := h.repository.CreateMessage(ctx, groupID, userID, text)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "メッセージを投稿できませんでした")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	groupID, userID, ok := requestIDs(w, r)
	if !ok {
		return
	}
	limit, offset, ok := pagination(w, r, 50)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if !h.requireMember(w, ctx, groupID, userID, "このグループのメッセージは閲覧できません") {
		return
	}
	items, err := h.repository.ListMessages(ctx, groupID, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "メッセージ一覧を取得できませんでした")
		return
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[1:]
	}
	writeJSON(w, http.StatusOK, MessageListResponse{Messages: items, Limit: limit, Offset: offset, HasMore: hasMore})
}

func (h *Handler) requireMember(w http.ResponseWriter, ctx context.Context, groupID, userID int64, forbidden string) bool {
	member, err := h.repository.IsMember(ctx, groupID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "グループの参加状態を確認できませんでした")
		return false
	}
	if !member {
		writeError(w, http.StatusForbidden, forbidden)
		return false
	}
	return true
}

func uniqueNames(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

func requestIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || groupID <= 0 {
		writeError(w, http.StatusBadRequest, "グループIDが不正です")
		return 0, 0, false
	}
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "ログインが必要です")
		return 0, 0, false
	}
	return groupID, userID, true
}

func pagination(w http.ResponseWriter, r *http.Request, fallback int) (int, int, bool) {
	limit, err := queryInt(r, "limit", fallback)
	if err != nil || limit < 1 || limit > 100 {
		writeError(w, http.StatusBadRequest, "limitは1以上100以下で指定してください")
		return 0, 0, false
	}
	offset, err := queryInt(r, "offset", 0)
	if err != nil || offset < 0 {
		writeError(w, http.StatusBadRequest, "offsetは0以上で指定してください")
		return 0, 0, false
	}
	return limit, offset, true
}

func queryInt(r *http.Request, key string, fallback int) (int, error) {
	if r.URL.Query().Get(key) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
