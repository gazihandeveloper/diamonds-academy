package admin

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/diamondsacademy/diamonds/internal/days"
	"github.com/diamondsacademy/diamonds/internal/views/pages"
)

const uploadDir = "web/static/uploads"
const maxUpload = 32 << 20 // 32 MB

func (h *Handler) DaysList(w http.ResponseWriter, r *http.Request) {
	list, err := h.Days.List(r.Context())
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	flash := r.URL.Query().Get("flash")
	render(w, r, pages.DaysList(pages.DaysListProps{Days: list, Flash: flash}))
}

func (h *Handler) DayNewGet(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.DayForm(pages.DayFormProps{
		IsEdit:    false,
		DayNo:     nextDayNo(r, h),
		Published: true,
	}))
}

func (h *Handler) DayNewPost(w http.ResponseWriter, r *http.Request) {
	in, props, ok := parseDayForm(r, false, 0, "")
	if !ok {
		render(w, r, pages.DayForm(props))
		return
	}
	if _, err := h.Days.Create(r.Context(), in); err != nil {
		props.Error = friendlyDBError(err)
		render(w, r, pages.DayForm(props))
		return
	}
	http.Redirect(w, r, "/admin/days?flash=Eğitim+oluşturuldu", http.StatusSeeOther)
}

func (h *Handler) DayEditGet(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	d, err := h.Days.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	render(w, r, pages.DayForm(pages.DayFormProps{
		IsEdit:      true,
		ID:          d.ID,
		DayNo:       d.DayNo,
		Title:       d.Title,
		BulletsText: strings.Join(d.Bullets, "\n"),
		Description: d.Description,
		Published:   d.Published,
		Video1URL:   d.Video1URL,
		Video2URL:   d.Video2URL,
		Video3URL:   d.Video3URL,
		FilePath:    d.FilePath,
		QuizText:    d.QuizText,
		QuizJSON:    d.QuizJSON,
	}))
}

func (h *Handler) DayEditPost(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	existingFile := ""
	if d, err := h.Days.GetByID(r.Context(), id); err == nil {
		existingFile = d.FilePath
	}
	in, props, ok := parseDayForm(r, true, id, existingFile)
	if !ok {
		render(w, r, pages.DayForm(props))
		return
	}
	if err := h.Days.Update(r.Context(), id, in); err != nil {
		if errors.Is(err, days.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		props.Error = friendlyDBError(err)
		render(w, r, pages.DayForm(props))
		return
	}
	http.Redirect(w, r, "/admin/days?flash=Eğitim+güncellendi", http.StatusSeeOther)
}

func (h *Handler) DayDelete(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	if err := h.Days.Delete(r.Context(), id); err != nil && !errors.Is(err, days.ErrNotFound) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/days?flash=Eğitim+silindi", http.StatusSeeOther)
}

// --- helpers ---

func idParam(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id
}

func nextDayNo(r *http.Request, h *Handler) int {
	list, err := h.Days.List(r.Context())
	if err != nil || len(list) == 0 {
		return 1
	}
	max := 0
	for _, d := range list {
		if d.DayNo > max {
			max = d.DayNo
		}
	}
	return max + 1
}

func parseDayForm(r *http.Request, isEdit bool, id int64, existingFile string) (days.Input, pages.DayFormProps, bool) {
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		// ParseMultipartForm content-type multipart değilse hata verir; klasik form için fallback
		if err := r.ParseForm(); err != nil {
			return days.Input{}, pages.DayFormProps{IsEdit: isEdit, ID: id, Error: "Form okunamadı."}, false
		}
	}
	dayNo, _ := strconv.Atoi(r.FormValue("day_no"))
	title := strings.TrimSpace(r.FormValue("title"))
	bulletsText := r.FormValue("bullets")
	desc := strings.TrimSpace(r.FormValue("description"))
	v1 := strings.TrimSpace(r.FormValue("video1_url"))
	v2 := strings.TrimSpace(r.FormValue("video2_url"))
	v3 := strings.TrimSpace(r.FormValue("video3_url"))
	quiz := r.FormValue("quiz_text")
	quizJSON := r.FormValue("quiz_json")
	published := r.FormValue("published") == "1"
	removeFile := r.FormValue("remove_file") == "1"

	filePath := existingFile
	if removeFile {
		filePath = ""
	}

	props := pages.DayFormProps{
		IsEdit:      isEdit,
		ID:          id,
		DayNo:       dayNo,
		Title:       title,
		BulletsText: bulletsText,
		Description: desc,
		Published:   published,
		Video1URL:   v1,
		Video2URL:   v2,
		Video3URL:   v3,
		FilePath:    filePath,
		QuizText:    quiz,
		QuizJSON:    quizJSON,
	}

	// PDF upload (opsiyonel)
	if r.MultipartForm != nil {
		if fh, _, err := r.FormFile("file"); err == nil {
			defer fh.Close()
			saved, err := saveUpload(r, "file")
			if err != nil {
				props.Error = "Dosya yüklenemedi: " + err.Error()
				return days.Input{}, props, false
			}
			if saved != "" {
				filePath = saved
				props.FilePath = saved
			}
		}
	}

	if dayNo < 1 {
		props.Error = "Eğitim numarası 1'den küçük olamaz."
		return days.Input{}, props, false
	}
	if title == "" {
		props.Error = "Başlık zorunludur."
		return days.Input{}, props, false
	}

	in := days.Input{
		DayNo:       dayNo,
		Title:       title,
		Bullets:     splitLines(bulletsText),
		Description: desc,
		Published:   published,
		Video1URL:   v1,
		Video2URL:   v2,
		Video3URL:   v3,
		FilePath:    filePath,
		QuizText:    quiz,
		QuizJSON:    quizJSON,
	}
	return in, props, true
}

// saveUpload, multipart formdan field adındaki dosyayı web/static/uploads altına kaydeder
// ve URL yolunu (/static/uploads/xxx.pdf) döner. Dosya yoksa "" döner.
func saveUpload(r *http.Request, field string) (string, error) {
	file, hdr, err := r.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return "", nil
		}
		return "", err
	}
	defer file.Close()
	if hdr.Size == 0 {
		return "", nil
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	if ext == "" {
		ext = ".pdf"
	}
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	name := hex.EncodeToString(buf) + ext
	dst, err := os.Create(filepath.Join(uploadDir, name))
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}
	return "/static/uploads/" + name, nil
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	parts := strings.Split(s, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func friendlyDBError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(msg, "UNIQUE") {
		return "Bu eğitim numarası zaten kullanılıyor."
	}
	return "Kaydedilemedi: " + msg
}
