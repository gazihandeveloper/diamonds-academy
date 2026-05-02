package pages

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"github.com/diamondsacademy/diamonds/internal/days"
)

func itoa(n int) string  { return strconv.Itoa(n) }

func itoa2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func progressPct(current, total int) string {
	if total <= 0 {
		return "0%"
	}
	pct := float64(current) / float64(total) * 100.0
	return fmt.Sprintf("%.2f%%", pct)
}

func joinBullets(b []string) string {
	if len(b) == 0 {
		return "—"
	}
	return strings.Join(b, " · ")
}

func formTitle(isEdit bool) string {
	if isEdit {
		return "Eğitim Düzenle"
	}
	return "Yeni Eğitim"
}

func formAction(p DayFormProps) templ.SafeURL {
	if p.IsEdit {
		return templ.SafeURL("/admin/days/" + itoa(int(p.ID)) + "/edit")
	}
	return templ.SafeURL("/admin/days/new")
}

func indexOfDay(list []days.Day, dayNo int) int {
	for i, d := range list {
		if d.DayNo == dayNo {
			return i
		}
	}
	return 0
}

func dayProgress(list []days.Day, current int) string {
	n := len(list)
	if n <= 0 {
		return "0%"
	}
	return progressPct(indexOfDay(list, current)+1, n)
}

func tabTitle(key, dayTitle string) string {
	switch key {
	case "l1":
		return "Ders 01"
	case "l2":
		return "Ders 02"
	case "l3":
		return "Ders 03"
	case "file":
		return "Özel Dosya"
	case "quiz":
		return "Quiz"
	}
	return dayTitle
}
