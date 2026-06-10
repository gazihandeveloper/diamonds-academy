package steps

import "github.com/diamondsacademy/diamonds/internal/days"

// Step represents a single numbered step in the learning path.
// Each step is either a video or a quiz, flattened from the day structure.
type Step struct {
	Number   int    // 1-based display number (Adım 1, Adım 2, ...)
	DayNo    int    // day_no from the days table
	Slot     string // "l1", "l2", "l3", or "quiz"
	Kind     string // "video" or "quiz"
	VideoURL string // populated for video steps
	Title    string // day title for context
}

// Build flattens all published days into a sequential step list.
// Only slots with actual content (non-empty video URL or quiz JSON) become steps.
// Order: all l1 of day 1, l2 of day 1, l3 of day 1, quiz of day 1,
//
//	l1 of day 2, l2 of day 2, ...
func Build(dayList []days.Day) []Step {
	var steps []Step
	n := 1
	for _, d := range dayList {
		if !d.Published {
			continue
		}
		if d.Video1URL != "" {
			steps = append(steps, Step{Number: n, DayNo: d.DayNo, Slot: "l1", Kind: "video", VideoURL: d.Video1URL, Title: d.Title})
			n++
		}
		if d.Video2URL != "" {
			steps = append(steps, Step{Number: n, DayNo: d.DayNo, Slot: "l2", Kind: "video", VideoURL: d.Video2URL, Title: d.Title})
			n++
		}
		if d.Video3URL != "" {
			steps = append(steps, Step{Number: n, DayNo: d.DayNo, Slot: "l3", Kind: "video", VideoURL: d.Video3URL, Title: d.Title})
			n++
		}
		if hasQuiz(d) {
			steps = append(steps, Step{Number: n, DayNo: d.DayNo, Slot: "quiz", Kind: "quiz", Title: d.Title})
			n++
		}
	}
	return steps
}

// FindByNumber returns the step with the given 1-based number, or nil.
func FindByNumber(steps []Step, num int) *Step {
	for i := range steps {
		if steps[i].Number == num {
			return &steps[i]
		}
	}
	return nil
}

// Total returns the number of content steps (excluding certificate).
func Total(steps []Step) int {
	return len(steps)
}

// hasQuiz returns true if any locale has quiz content.
func hasQuiz(d days.Day) bool {
	return d.QuizJSON != "" || d.QuizJSON_EN != "" || d.QuizJSON_BG != ""
}
