package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

type AnswerTemplateHandler struct {
	store store.Interface
}

func (h *AnswerTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	templates, err := h.store.ListAnswerTemplates(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"templates": templates})
}

func (h *AnswerTemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var t models.AnswerTemplate
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	t.CurrentVersion = 1
	if err := h.store.CreateAnswerTemplate(r.Context(), &t); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Create initial version snapshot
	if err := h.store.CreateAnswerTemplateVersion(r.Context(), &models.AnswerTemplateVersion{
		TemplateID:  t.ID,
		Version:     1,
		Content:     t.Content,
		Description: "初始版本",
	}); err != nil {
		// Rollback template creation if version snapshot fails
		h.store.DeleteAnswerTemplate(r.Context(), t.ID)
		Error(w, http.StatusInternalServerError, "版本快照创建失败")
		return
	}
	Created(w, t)
}

func (h *AnswerTemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	t, err := h.store.GetAnswerTemplate(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, "模板未找到")
		return
	}
	OK(w, t)
}

func (h *AnswerTemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	var t models.AnswerTemplate
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	t.ID = uint(id)

	// Fetch current template to detect content change
	old, err := h.store.GetAnswerTemplate(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, "模板未找到")
		return
	}

	if old.Content != t.Content {
		// Bump version and create snapshot
		t.CurrentVersion = old.CurrentVersion + 1
		if err := h.store.CreateAnswerTemplateVersion(r.Context(), &models.AnswerTemplateVersion{
			TemplateID:  t.ID,
			Version:     t.CurrentVersion,
			Content:     t.Content,
			Description: t.Description,
		}); err != nil {
			Error(w, http.StatusInternalServerError, "版本快照创建失败")
			return
		}
	} else {
		t.CurrentVersion = old.CurrentVersion
	}

	if err := h.store.UpdateAnswerTemplate(r.Context(), &t); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, t)
}

func (h *AnswerTemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	if err := h.store.DeleteAnswerTemplate(r.Context(), uint(id)); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = h.store.DeleteAnswerTemplateVersions(r.Context(), uint(id))
	w.WriteHeader(http.StatusNoContent)
}

// ListVersions returns all versions for a template.
func (h *AnswerTemplateHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	versions, err := h.store.ListAnswerTemplateVersions(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"versions": versions})
}

// GetVersion returns a specific version's content.
func (h *AnswerTemplateHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	ver, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的版本号")
		return
	}
	v, err := h.store.GetAnswerTemplateVersion(r.Context(), uint(id), ver)
	if err != nil {
		Error(w, http.StatusNotFound, "版本未找到")
		return
	}
	OK(w, v)
}

// Rollback rolls back to a specific version.
func (h *AnswerTemplateHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	ver, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的版本号")
		return
	}

	// Fetch the old version
	v, err := h.store.GetAnswerTemplateVersion(r.Context(), uint(id), ver)
	if err != nil {
		Error(w, http.StatusNotFound, "版本未找到")
		return
	}

	// Fetch current template
	t, err := h.store.GetAnswerTemplate(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, "模板未找到")
		return
	}

	// Snapshot current content before rollback
	newVer := t.CurrentVersion + 1
	if err := h.store.CreateAnswerTemplateVersion(r.Context(), &models.AnswerTemplateVersion{
		TemplateID:  t.ID,
		Version:     newVer,
		Content:     t.Content,
		Description: "回滚到版本 " + strconv.Itoa(ver),
	}); err != nil {
		Error(w, http.StatusInternalServerError, "版本快照创建失败")
		return
	}

	// Restore old version content
	t.Content = v.Content
	t.CurrentVersion = newVer
	if err := h.store.UpdateAnswerTemplate(r.Context(), t); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, t)
}
